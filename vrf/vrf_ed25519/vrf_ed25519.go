/**
 * @license
 * Copyright 2017 Yahoo Inc. All rights reserved.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package vrf_ed25519

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"math/big"
//	"fmt"
//	"encoding/hex"

	"golang.org/x/crypto/ed25519"
	"github.com/yahoo/coname/vrf/vrf_ed25519/golang_x_crypt_ed25519_internal/edwards25519"	// for now. It has to be "golang.org/x/crypto/ed25519/internal/edwards25519"
)

const (
	limit = 10000
	N2 = 32		// ceil(log2(q) / 8)
	N = N2/2
	qs = "1000000000000000000000000000000014def9dea2f79cd65812631a5cf5d3ed"	// 2^252 + 27742317777372353535851937790883648493
)

var (
	ErrMalformedInput = errors.New("ECVRF: malformed input")
	ErrDecodeError = errors.New("ECVRF: decode error")
	ErrInternalError = errors.New("ECVRF: internal error")
	q, _ = new(big.Int).SetString(qs, 16)
	g = G()
)

const (
	// PublicKeySize is the size, in bytes, of public keys as used in this package.
	PublicKeySize = 32
	// PrivateKeySize is the size, in bytes, of private keys as used in this package.
	PrivateKeySize = 64
	// SignatureSize is the size, in bytes, of signatures generated and verified by this package.
	SignatureSize = 64
)

// assume <pk, sk> were generated by ed25519.GenerateKey()
func ECVRF_prove(pk []byte, sk []byte, m []byte) (pi []byte, err error) {
	x := expandSecret(sk)
	h := ECVRF_hash_to_curve(m, pk)
	r := ECP2OS(GeScalarMult(h, x))

	kp, ks, err := ed25519.GenerateKey(nil)	// use GenerateKey to generate a random
	if err != nil {
		return nil, err
	}
	k := expandSecret(ks)

	// ECVRF_hash_points(g, h, g^x, h^x, g^k, h^k)
	c := ECVRF_hash_points(ECP2OS(g), ECP2OS(h), S2OS(pk), r, S2OS(kp), ECP2OS(GeScalarMult(h, k)))

	// s = k - c*x mod q
	var z big.Int
	s := z.Mod(z.Sub(F2IP(k), z.Mul(c, F2IP(x))), q)

	// pi = gamma || I2OSP(c, N) || I2OSP(s, 2N)
	var buf bytes.Buffer
	buf.Write(r)		// 2N
	buf.Write(I2OSP(c, N))
	buf.Write(I2OSP(s, N2))
	return buf.Bytes(), nil
}

func ECVRF_proof2hash(pi []byte) []byte {
	return pi[1 : N2 + 1]
}

func ECVRF_verify(pk []byte, pi []byte, m []byte) (bool, error) {
	r, c, s, err := ECVRF_decode_proof(pi)
	if err != nil {
		return false, err
	}

	// u = (g^x)^c * g^s = P^c * g^s
	var u edwards25519.ProjectiveGroupElement
	P := OS2ECP(pk, pk[31] >> 7)
	if P == nil {
		return false, ErrMalformedInput
	}
	edwards25519.GeDoubleScalarMultVartime(&u, c, P, s)

	h := ECVRF_hash_to_curve(m, pk)

	// v = gamma^c * h^s
//	fmt.Printf("c, r, s, h\n%s%s%s%s\n", hex.Dump(c[:]), hex.Dump(ECP2OS(r)), hex.Dump(s[:]), hex.Dump(ECP2OS(h)))
	v := GeAdd(GeScalarMult(r, c), GeScalarMult(h, s))

	// c' = ECVRF_hash_points(g, h, g^x, gamma, u, v)
	c2 := ECVRF_hash_points(ECP2OS(g), ECP2OS(h), S2OS(pk), ECP2OS(r), ECP2OSProj(&u), ECP2OS(v))

	return c2.Cmp(F2IP(c)) == 0, nil
}

func ECVRF_decode_proof(pi []byte) (r *edwards25519.ExtendedGroupElement, c *[N2]byte, s *[N2]byte, err error) {
	i := 0
	sign := pi[i]; i++
	if sign != 2 && sign != 3 {
		return nil, nil, nil, ErrDecodeError
	}
	r = OS2ECP(pi[i:i+N2], sign - 2); i += N2
	if r == nil {
		return nil, nil, nil, ErrDecodeError
	}

	// swap and expand to make it a field
	c = new([N2]byte)
	for j := N - 1; j >= 0; j-- {
		c[j] = pi[i]
		i++
	}

	// swap to make it a field
	s = new([N2]byte)
	for j := N2 - 1; j >= 0; j-- {
		s[j] = pi[i]
		i++
	}
	return
}

func ECVRF_hash_points(ps ... []byte) *big.Int {
	h := sha256.New()
//	fmt.Printf("hash_points:\n")
	for _, p := range ps {
		h.Write(p)
//		fmt.Printf("%s\n", hex.Dump(p))
	}
	v := h.Sum(nil)
	return OS2IP(v[:N])
}

func ECVRF_hash_to_curve(m []byte, pk []byte) *edwards25519.ExtendedGroupElement {
	hash := sha256.New()
	for i := int64(0); i < limit; i++ {
		ctr := I2OSP(big.NewInt(i), 4)
		hash.Write(m)
		hash.Write(pk)
		hash.Write(ctr)
		h := hash.Sum(nil)
		hash.Reset()
		if P := OS2ECP(h, 0); P != nil {
			return P
		}
		if P := OS2ECP(h, 1); P != nil {
			return P
		}
	}
	panic("ECVRF_hash_to_curve: couldn't make a point on curve")
}

func OS2ECP(os []byte, sign byte) *edwards25519.ExtendedGroupElement {
	P := new(edwards25519.ExtendedGroupElement)
	var buf [32]byte
	copy(buf[:], os)
	buf[31] = (sign << 7) | (buf[31] & 0x7f)
	if !P.FromBytes(&buf) {
		return nil
	}
	var t [32]byte
	inf := GeScalarMult(P, IP2F(q))
	inf.ToBytes(&t)
	if t != [32]byte{1} {
//		fmt.Printf("OS2ECP: not valid curve\n")
		return nil
	}
	return P
}

// just prepend the sign octet
func S2OS(s []byte) []byte {
	sign := s[31] >> 7		// @@ we should clear the sign bit??
	os := []byte{sign + 2}		// Y = 0x02 if positive or 0x03 if negative
	os = append([]byte(os), s...)
	return os
}

func ECP2OS(P *edwards25519.ExtendedGroupElement) []byte {
	var s [32]byte
	P.ToBytes(&s)
	return S2OS(s[:])
}

func ECP2OSProj(P *edwards25519.ProjectiveGroupElement) []byte {
	var s [32]byte
	P.ToBytes(&s)
	return S2OS(s[:])
}

func I2OSP(b *big.Int, n int) []byte {
	os := b.Bytes()
	if n > len(os) {
		var buf bytes.Buffer
		buf.Write(make([]byte, n - len(os)))	// prepend 0s
		buf.Write(os)
		return buf.Bytes()
	} else {
		return os[:n]
	}
}

func OS2IP(os []byte) *big.Int {
	return new(big.Int).SetBytes(os)
}

// convert a field number (in LittleEndian) to a big int
func F2IP(f *[32]byte) *big.Int {
	var t [32]byte
	for i := 0; i < 32; i++ {
		t[32 - i - 1] = f[i]
	}
	return OS2IP(t[:])
}

func IP2F(b *big.Int) *[32]byte {
	os := b.Bytes()
	r := new([32]byte)
	j := len(os) - 1
	for i := 0; i < 32 && j >= 0; i++ {
		r[i] = os[j]
		j--
	}
	return r
}

func GeScalarMult(h *edwards25519.ExtendedGroupElement, a *[32]byte) *edwards25519.ExtendedGroupElement {
	r := new(edwards25519.ExtendedGroupElement)
	var pg edwards25519.ProjectiveGroupElement
	edwards25519.GeDoubleScalarMultVartime(&pg, a, h, &[32]byte{})	// h^a * g^0
	var t [32]byte
	pg.ToBytes(&t)
	r.FromBytes(&t)
	return r
}

func G() *edwards25519.ExtendedGroupElement {
	g := new(edwards25519.ExtendedGroupElement)
	var f edwards25519.FieldElement
	edwards25519.FeOne(&f)
	var s [32]byte
	edwards25519.FeToBytes(&s, &f)
	edwards25519.GeScalarMultBase(g, &s)	// g = g^1
	return g
}

func expandSecret(sk []byte) *[32]byte {
	// copied from golang.org/x/crypto/ed25519/ed25519.go -- has to be the same
	digest := sha512.Sum512(sk[:32])
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64
	h := new([32]byte)
	copy(h[:], digest[:])
	return h
}

//
// copied from edwards25519.go and const.go in golang.org/x/crypto/ed25519/internal/edwards25519
//
type CachedGroupElement struct {
	yPlusX, yMinusX, Z, T2d edwards25519.FieldElement
}

// d2 is 2*d.
var d2 = edwards25519.FieldElement{
	-21827239, -5839606, -30745221, 13898782, 229458, 15978800, -12551817, -6495438, 29715968, 9444199,
}

func ToCached(r *CachedGroupElement, p *edwards25519.ExtendedGroupElement) {
	edwards25519.FeAdd(&r.yPlusX, &p.Y, &p.X)
	edwards25519.FeSub(&r.yMinusX, &p.Y, &p.X)
	edwards25519.FeCopy(&r.Z, &p.Z)
	edwards25519.FeMul(&r.T2d, &p.T, &d2)
}

func GeAdd(p, qe *edwards25519.ExtendedGroupElement) *edwards25519.ExtendedGroupElement {
	var q CachedGroupElement
	var r edwards25519.CompletedGroupElement
	var t0 edwards25519.FieldElement

	ToCached(&q, qe)

	edwards25519.FeAdd(&r.X, &p.Y, &p.X)
	edwards25519.FeSub(&r.Y, &p.Y, &p.X)
	edwards25519.FeMul(&r.Z, &r.X, &q.yPlusX)
	edwards25519.FeMul(&r.Y, &r.Y, &q.yMinusX)
	edwards25519.FeMul(&r.T, &q.T2d, &p.T)
	edwards25519.FeMul(&r.X, &p.Z, &q.Z)
	edwards25519.FeAdd(&t0, &r.X, &r.X)
	edwards25519.FeSub(&r.X, &r.Z, &r.Y)
	edwards25519.FeAdd(&r.Y, &r.Z, &r.Y)
	edwards25519.FeAdd(&r.Z, &t0, &r.T)
	edwards25519.FeSub(&r.T, &t0, &r.T)

	re := new(edwards25519.ExtendedGroupElement)
	r.ToExtended(re)
	return re
}
