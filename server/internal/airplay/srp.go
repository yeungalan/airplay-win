package airplay

import (
	"crypto/rand"
	"crypto/sha512"
	"math/big"
)

// SRP-6a implementation for AirPlay/HomeKit PIN pairing.
// Uses RFC 5054 3072-bit group with SHA-512 and username "Pair-Setup".

// RFC 5054 Appendix A - 3072-bit MODP group, generator g=5
const srpNHex = "FFFFFFFFFFFFFFFFC90FDAA22168C234" +
	"C4C6628B80DC1CD129024E088A67CC74" +
	"020BBEA63B139B22514A08798E3404DD" +
	"EF9519B3CD3A431B302B0A6DF25F1437" +
	"4FE1356D6D51C245E485B576625E7EC6" +
	"F44C42E9A637ED6B0BFF5CB6F406B7ED" +
	"EE386BFB5A899FA5AE9F24117C4B1FE6" +
	"49286651ECE45B3DC2007CB8A163BF05" +
	"98DA48361C55D39A69163FA8FD24CF5F" +
	"83655D23DCA3AD961C62F356208552BB" +
	"9ED529077096966D670C354E4ABC9804" +
	"F1746C08CA18217C32905E462E36CE3B" +
	"E39E772C180E86039B2783A2EC07A28F" +
	"B5C55DF06F4C52C9DE2BCBF695581718" +
	"3995497CEA956AE515D2261898FA0510" +
	"15728E5A8AAAC42DAD33170D04507A33" +
	"A85521ABDF1CBA64ECFB850458DBEF0A" +
	"8AEA71575D060C7DB3970F85A6E1E4C7" +
	"ABF5AE8CDB0933D71E8C94E04A25619D" +
	"CEE3D2261AD2EE6BF12FFA06D98A0864" +
	"D87602733EC86A64521F2B18177B200C" +
	"BBE117577A615D6C770988C0BAD946E2" +
	"08E24FA074E5AB3143DB5BFCE0FD108E" +
	"4B82D120A93AD2CAFFFFFFFFFFFFFFFF"

var (
	srpN *big.Int
	srpG = big.NewInt(5)
	srpK *big.Int // k = H(N || pad(g))
)

func initSRP() {
	srpN, _ = new(big.Int).SetString(srpNHex, 16)
	nBytes := srpPad(srpN)
	gPad := make([]byte, len(nBytes))
	gB := srpG.Bytes()
	copy(gPad[len(gPad)-len(gB):], gB)
	h := sha512.New()
	h.Write(nBytes)
	h.Write(gPad)
	srpK = new(big.Int).SetBytes(h.Sum(nil))
}

// srpPad pads v to the byte-length of N (384 bytes).
func srpPad(v *big.Int) []byte {
	nLen := (srpN.BitLen() + 7) / 8
	b := v.Bytes()
	if len(b) >= nLen {
		return b[len(b)-nLen:]
	}
	out := make([]byte, nLen)
	copy(out[nLen-len(b):], b)
	return out
}

func srpHash(parts ...[]byte) []byte {
	h := sha512.New()
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

// srpNewSalt returns a fresh 16-byte random salt.
func srpNewSalt() []byte {
	s := make([]byte, 16)
	rand.Read(s)
	return s
}

// srpVerifier computes v = g^x mod N where x = H(salt || H("Pair-Setup:" || pin)).
func srpVerifier(salt []byte, pin string) *big.Int {
	inner := srpHash([]byte("Pair-Setup:" + pin))
	x := new(big.Int).SetBytes(srpHash(salt, inner))
	return new(big.Int).Exp(srpG, x, srpN)
}

// srpServerKeys generates server private key b and public key B = (k*v + g^b) mod N.
func srpServerKeys(v *big.Int) (bPriv, bPub *big.Int) {
	bBytes := make([]byte, 64)
	rand.Read(bBytes)
	bPriv = new(big.Int).SetBytes(bBytes)
	kv := new(big.Int).Mul(srpK, v)
	kv.Mod(kv, srpN)
	gb := new(big.Int).Exp(srpG, bPriv, srpN)
	bPub = new(big.Int).Add(kv, gb)
	bPub.Mod(bPub, srpN)
	return
}

// srpSessionKey computes session key K = H(S) where S = (A * v^u)^b mod N.
// Returns nil if A is invalid (A mod N == 0).
func srpSessionKey(A, bPub, bPriv, v *big.Int) []byte {
	if new(big.Int).Mod(A, srpN).Sign() == 0 {
		return nil
	}
	u := new(big.Int).SetBytes(srpHash(srpPad(A), srpPad(bPub)))
	vu := new(big.Int).Exp(v, u, srpN)
	avu := new(big.Int).Mul(A, vu)
	avu.Mod(avu, srpN)
	S := new(big.Int).Exp(avu, bPriv, srpN)
	return srpHash(srpPad(S))
}

// srpServerProof computes M2 = H(pad(A) || M1 || K).
func srpServerProof(A *big.Int, m1, K []byte) []byte {
	return srpHash(srpPad(A), m1, K)
}
