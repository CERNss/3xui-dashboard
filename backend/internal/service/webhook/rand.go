package webhook

import "crypto/rand"

// cryptoRandReader is a package-level handle so tests can swap in a
// deterministic reader if needed.
var cryptoRandReader = rand.Reader
