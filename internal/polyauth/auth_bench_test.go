package polyauth

import "testing"

func BenchmarkHMACSignature(b *testing.B) {
	body := []byte(`{"hash":"0x123","owner":"0xabc","side":"BUY","price":"0.45","size":"10"}`)
	decoded, _ := DecodeAPISecret("c2VjcmV0")

	b.Run("double_quotes", func(b *testing.B) {
		for b.Loop() {
			_ = HMACSignatureBytes(
				decoded,
				1710000000,
				"POST",
				"/order",
				body,
			)
		}
	})

	b.Run("single_quotes", func(b *testing.B) {
		body := []byte("{'hash':'0x123','owner':'0xabc','side':'BUY','price':'0.45','size':'10'}")
		for b.Loop() {
			_ = HMACSignatureBytes(
				decoded,
				1710000000,
				"POST",
				"/order",
				body,
			)
		}
	})
}
