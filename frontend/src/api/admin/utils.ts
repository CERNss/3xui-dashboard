import { adminClient } from '../client/admin'

export interface X25519Keypair {
  privateKey: string
  publicKey: string
}

export const utilsApi = {
  x25519: () => adminClient.post<X25519Keypair>('/utils/x25519').then((r) => r.data),
}
