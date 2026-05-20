package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

// WGProvisioner implements the dashboard side of WireGuard peer
// add/remove. It is intentionally separate from the unified
// ClientService.Provision path because the wire shape is entirely
// different: the panel stores all peer state in inbound.settings.peers[],
// so every mutation is a read-modify-write on that JSON blob
// rather than a per-client API call.
//
// Concurrency model: each mutation runs inside a Postgres tx that
// holds pg_advisory_xact_lock(inbound_id), which serializes
// dashboard-side RMW cycles on the same inbound. The lock does
// NOT extend to mutations made directly through the 3x-ui panel
// UI — see docs/operator/wireguard-setup.md.
type WGProvisioner struct {
	rt        *runtime.Manager
	ownership *repository.ClientOwnershipRepo
	peers     *repository.WGPeerRepo
	cipher    *wgcrypto.Cipher
}

// NewWGProvisioner constructs a provisioner. cipher MUST be
// initialised from WG_MASTER_KEY at startup; a nil cipher is
// rejected so we don't silently store plaintext keys.
func NewWGProvisioner(
	rt *runtime.Manager,
	ownership *repository.ClientOwnershipRepo,
	peers *repository.WGPeerRepo,
	cipher *wgcrypto.Cipher,
) (*WGProvisioner, error) {
	if rt == nil || ownership == nil || peers == nil {
		return nil, errors.New("WGProvisioner: rt + ownership + peers are required")
	}
	if cipher == nil {
		return nil, errors.New("WGProvisioner: cipher is required (WG_MASTER_KEY missing?)")
	}
	return &WGProvisioner{rt: rt, ownership: ownership, peers: peers, cipher: cipher}, nil
}

// ProvisionPeer is the WG analogue of ClientService.ProvisionClient.
// Behaviour:
//   - First call:  generates a keypair, allocates the next-free
//     IP inside the inbound's subnet, appends a peer to the
//     remote inbound's settings.peers[] via UpdateInboundByID,
//     and writes both the ownership row and the wg_peers mirror
//     in one transaction. PlanParams (expiry + limit) is applied
//     in the same Save — no second Upsert pass needed.
//   - Re-provisioning the same ownership extends the existing row
//     with the new expiry window but does not mutate the node.
//
// Returns the persisted ClientOwnership row plus the WGPeer row
// the caller needs to render the .conf for the user.
func (p *WGProvisioner) ProvisionPeer(ctx context.Context, userID, nodeID int64, inboundTag, clientEmail string, params PlanParams) (*model.ClientOwnership, *model.WGPeer, error) {
	r, err := p.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	inbound, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInboundLookup, err)
	}
	if !inbound.IsWireguard() {
		return nil, nil, fmt.Errorf("WGProvisioner: inbound %q is %s, not wireguard", inboundTag, inbound.Protocol)
	}

	// Short-circuit re-provisioning: if an ownership + peer already
	// exist for this triple, return them. Re-keying is a separate
	// flow.
	existingOwn, err := p.ownership.GetByTriple(ctx, nodeID, inboundTag, clientEmail)
	if err != nil {
		return nil, nil, err
	}
	if existingOwn != nil {
		peer, err := p.peers.GetByOwnership(ctx, existingOwn.ID)
		if err != nil {
			return nil, nil, err
		}
		if peer != nil {
			return existingOwn, peer, nil
		}
		// Ownership exists but mirror row is missing — a previous
		// provision attempt half-finished. Continue to recreate the
		// peer (idempotent on the panel side via key match in WG).
	}

	keypair, err := wgcrypto.GenerateKeypair()
	if err != nil {
		return nil, nil, err
	}
	sealedPriv, err := p.cipher.Seal([]byte(keypair.Private))
	if err != nil {
		return nil, nil, err
	}

	var (
		ownershipOut *model.ClientOwnership
		peerOut      *model.WGPeer
	)
	err = p.peers.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.AdvisoryLock(ctx, tx, inbound.ID); err != nil {
			return err
		}
		// Refresh inbound state under the lock — another tx may
		// have mutated peers[] since we read above.
		fresh, err := r.GetInbound(ctx, inboundTag)
		if err != nil {
			return err
		}
		var settings runtime.WGSettings
		if fresh.Settings != "" {
			if err := json.Unmarshal([]byte(fresh.Settings), &settings); err != nil {
				return fmt.Errorf("decode WG settings: %w", err)
			}
		}

		taken, err := p.peers.AllocatedIPs(ctx, tx, fresh.ID)
		if err != nil {
			return err
		}
		// Also mark the addresses already on the panel as taken —
		// the dashboard may have lost track of some.
		for _, peer := range settings.Peers {
			for _, cidr := range peer.AllowedIPs {
				if ip, ok := firstIPOfCIDR(cidr); ok {
					taken[ip] = struct{}{}
				}
			}
		}
		ipStr, err := allocateIP(taken)
		if err != nil {
			return err
		}

		newPeer := runtime.WGPeer{
			PrivateKey: keypair.Private,
			PublicKey:  keypair.Public,
			AllowedIPs: []string{ipStr + "/32"},
			KeepAlive:  25,
		}
		settings.Peers = append(settings.Peers, newPeer)
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		fresh.Settings = string(settingsJSON)
		if _, err := r.UpdateInboundByID(ctx, fresh.ID, fresh); err != nil {
			return fmt.Errorf("push inbound update: %w", err)
		}

		// Persist ownership + peer mirror in the same tx. The plan
		// window (expiry + traffic limit) is baked into the initial
		// Save so we don't need a second Upsert pass after the tx
		// commits.
		newExpiry := computeExpiry(time.Now().UTC(), existingOwn, params.DurationDays)
		var expiresAt *time.Time
		if !newExpiry.IsZero() {
			v := newExpiry
			expiresAt = &v
		}
		var trafficLimit *int64
		if params.TrafficLimitBytes > 0 {
			v := params.TrafficLimitBytes
			trafficLimit = &v
		}
		row := &model.ClientOwnership{
			UserID:            userID,
			NodeID:            nodeID,
			InboundTag:        inboundTag,
			ClientEmail:       clientEmail,
			Protocol:          "wireguard",
			PlanID:            params.PlanID,
			ExpiresAt:         expiresAt,
			TrafficLimitBytes: trafficLimit,
			Enabled:           true,
		}
		if existingOwn != nil {
			row.ID = existingOwn.ID
		}
		if err := tx.Save(row).Error; err != nil {
			return fmt.Errorf("save ownership: %w", err)
		}
		ownershipOut = row

		peer := &model.WGPeer{
			ClientOwnershipID:   row.ID,
			InboundID:           fresh.ID,
			PublicKey:           keypair.Public,
			PrivateKeyEncrypted: sealedPriv,
			AllocatedIP:         ipStr,
		}
		if err := p.peers.Create(ctx, tx, peer); err != nil {
			return err
		}
		peerOut = peer
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return ownershipOut, peerOut, nil
}

// RemovePeer is the WG analogue of DeleteClient. Removes the peer
// from the remote inbound's settings.peers[] (matched by public
// key), clears the wg_peers mirror, and clears the ownership row.
// Idempotent: a missing ownership / peer / panel entry is
// treated as success.
func (p *WGProvisioner) RemovePeer(ctx context.Context, nodeID int64, inboundTag, clientEmail string) error {
	r, err := p.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	inbound, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return err
	}
	if !inbound.IsWireguard() {
		return fmt.Errorf("WGProvisioner.RemovePeer: inbound %q is %s, not wireguard", inboundTag, inbound.Protocol)
	}
	own, err := p.ownership.GetByTriple(ctx, nodeID, inboundTag, clientEmail)
	if err != nil {
		return err
	}
	if own == nil {
		return nil
	}
	mirror, err := p.peers.GetByOwnership(ctx, own.ID)
	if err != nil {
		return err
	}

	return p.peers.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.AdvisoryLock(ctx, tx, inbound.ID); err != nil {
			return err
		}
		fresh, err := r.GetInbound(ctx, inboundTag)
		if err != nil {
			return err
		}
		var settings runtime.WGSettings
		if fresh.Settings != "" {
			if err := json.Unmarshal([]byte(fresh.Settings), &settings); err != nil {
				return fmt.Errorf("decode WG settings: %w", err)
			}
		}
		if mirror != nil {
			filtered := settings.Peers[:0]
			for _, peer := range settings.Peers {
				if peer.PublicKey == mirror.PublicKey {
					continue
				}
				filtered = append(filtered, peer)
			}
			settings.Peers = filtered
			settingsJSON, err := json.Marshal(settings)
			if err != nil {
				return err
			}
			fresh.Settings = string(settingsJSON)
			if _, err := r.UpdateInboundByID(ctx, fresh.ID, fresh); err != nil {
				return fmt.Errorf("push inbound update: %w", err)
			}
		}
		if err := p.peers.DeleteByOwnership(ctx, tx, own.ID); err != nil {
			return err
		}
		return p.ownership.ClearForClient(ctx, nodeID, inboundTag, clientEmail)
	})
}

// DecryptPrivateKey returns the plaintext private key for a peer
// row. Used by the subscription renderer.
func (p *WGProvisioner) DecryptPrivateKey(peer *model.WGPeer) (string, error) {
	plain, err := p.cipher.Open(peer.PrivateKeyEncrypted)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// PeerView is the read-only WG peer surface the subscription
// renderer wants. Lives here so the sub package can depend on
// it without pulling in the GORM model. ServerPublicKey is left
// blank — the renderer derives it from the inbound's secretKey.
type PeerView struct {
	PrivateKey      string
	PublicKey       string
	ServerPublicKey string
	AllocatedIP     string
	MTU             int
}

// PeerForOwnership implements sub.WGPeerSource: returns the
// mirror peer with a pre-decrypted PrivateKey, or (nil, nil)
// when no peer exists for this ownership.
func (p *WGProvisioner) PeerForOwnership(ctx context.Context, ownershipID int64) (*PeerView, error) {
	peer, err := p.peers.GetByOwnership(ctx, ownershipID)
	if err != nil {
		return nil, err
	}
	if peer == nil {
		return nil, nil
	}
	priv, err := p.DecryptPrivateKey(peer)
	if err != nil {
		return nil, err
	}
	return &PeerView{
		PrivateKey:  priv,
		PublicKey:   peer.PublicKey,
		AllocatedIP: peer.AllocatedIP,
	}, nil
}

// ---- IP allocator ---------------------------------------------------------

// defaultSubnet is the per-inbound CIDR used when the inbound has
// no explicit subnet hint. Matches the 3x-ui fork default for
// WG-on-node deployments.
const defaultSubnet = "10.0.0.0/24"

// allocateIP picks the lowest free /32 inside defaultSubnet that is
// not in taken. The host address 10.0.0.1 is reserved for the
// node-side WG interface and SHALL NOT be handed out.
func allocateIP(taken map[string]struct{}) (string, error) {
	prefix, err := netip.ParsePrefix(defaultSubnet)
	if err != nil {
		return "", err
	}
	addr := prefix.Addr()
	end := lastAddrInPrefix(prefix)
	// Skip the network address (.0) and the gateway (.1).
	cur := addr.Next().Next()
	for cur.Compare(end) <= 0 {
		s := cur.String()
		if _, busy := taken[s]; !busy {
			return s, nil
		}
		cur = cur.Next()
	}
	return "", fmt.Errorf("WGProvisioner: subnet %s exhausted (%d taken)", defaultSubnet, len(taken))
}

// firstIPOfCIDR returns the host portion of a "10.0.0.42/32"-style
// allowed-IPs entry as a bare IP string, or false if unparseable.
func firstIPOfCIDR(s string) (string, bool) {
	if pfx, err := netip.ParsePrefix(s); err == nil {
		return pfx.Addr().String(), true
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a.String(), true
	}
	return "", false
}

// lastAddrInPrefix returns the broadcast/highest address inside
// pfx. Used as the upper bound of the IP allocator's scan.
func lastAddrInPrefix(pfx netip.Prefix) netip.Addr {
	bytes := pfx.Masked().Addr().As4()
	maskBits := pfx.Bits()
	hostBits := 32 - maskBits
	suffix := uint32(1<<hostBits) - 1
	combined := uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
	combined |= suffix
	return netip.AddrFrom4([4]byte{
		byte(combined >> 24), byte(combined >> 16), byte(combined >> 8), byte(combined),
	})
}
