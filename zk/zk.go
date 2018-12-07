package zk

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/arnaucube/go-snark/bn128"
	"github.com/arnaucube/go-snark/fields"
)

type Setup struct {
	Toxic struct {
		T  *big.Int // trusted setup secret
		Ka *big.Int // prover
		Kb *big.Int // prover
		Kc *big.Int // prover
	}

	// public
	G1T [][3]*big.Int    // t encrypted in G1 curve
	G2T [][3][2]*big.Int // t encrypted in G2 curve
}
type Proof struct {
	PiA  [3]*big.Int
	PiAp [3]*big.Int
	PiB  [3][2]*big.Int
	PiBp [3]*big.Int
	PiC  [3]*big.Int
	PiCp [3]*big.Int
	PiH  [3]*big.Int
	Va   [3][2]*big.Int
	Vb   [3]*big.Int
	Vc   [3][2]*big.Int
	Vz   [3][2]*big.Int
}

const bits = 512

func GenerateTrustedSetup(bn bn128.Bn128, pollength int) (Setup, error) {
	var setup Setup
	var err error
	// generate random t value
	setup.Toxic.T, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}
	fmt.Print("trusted t: ")
	fmt.Println(setup.Toxic.T)

	// encrypt t values with curve generators
	var gt1 [][3]*big.Int
	var gt2 [][3][2]*big.Int
	for i := 0; i < pollength; i++ {
		tPow := bn.Fq1.Exp(setup.Toxic.T, big.NewInt(int64(i)))
		tEncr1 := bn.G1.MulScalar(bn.G1.G, tPow)
		gt1 = append(gt1, tEncr1)
		tEncr2 := bn.G2.MulScalar(bn.G2.G, tPow)
		gt2 = append(gt2, tEncr2)
	}
	// gt1: g1, g1*t, g1*t^2, g1*t^3, ...
	// gt2: g2, g2*t, g2*t^2, ...
	setup.G1T = gt1
	setup.G2T = gt2

	return setup, nil
}

func GenerateProofs(bn bn128.Bn128, f fields.Fq, setup Setup, ax, bx, cx, hx, zx []*big.Int) (Proof, error) {
	var proof Proof
	var err error

	// k for calculating pi' and Vk
	setup.Toxic.Ka, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Proof{}, err
	}
	setup.Toxic.Kb, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Proof{}, err
	}
	setup.Toxic.Kc, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Proof{}, err
	}

	// g1*A(t)
	proof.PiA = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	for i := 0; i < len(ax); i++ {
		m := bn.G1.MulScalar(setup.G1T[i], ax[i])
		proof.PiA = bn.G1.Add(proof.PiA, m)
	}
	proof.PiAp = bn.G1.MulScalar(proof.PiA, setup.Toxic.Ka)

	// g2*B(t)
	proof.PiB = bn.Fq6.Zero()
	// g1*B(t)
	pib1 := [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	for i := 0; i < len(bx); i++ {
		m := bn.G2.MulScalar(setup.G2T[i], bx[i])
		proof.PiB = bn.G2.Add(proof.PiB, m)
		m1 := bn.G1.MulScalar(setup.G1T[i], bx[i])
		pib1 = bn.G1.Add(pib1, m1)
	}
	proof.PiBp = bn.G1.MulScalar(pib1, setup.Toxic.Kb)

	// g1*C(t)
	proof.PiC = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	for i := 0; i < len(cx); i++ {
		m := bn.G1.MulScalar(setup.G1T[i], cx[i])
		proof.PiC = bn.G1.Add(proof.PiC, m)
	}
	proof.PiCp = bn.G1.MulScalar(proof.PiC, setup.Toxic.Kc)

	// g1*H(t)
	proof.PiH = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	for i := 0; i < len(hx); i++ {
		m := bn.G1.MulScalar(setup.G1T[i], hx[i])
		proof.PiH = bn.G1.Add(proof.PiH, m)
	}

	g2Zt := bn.Fq6.Zero()
	for i := 0; i < len(bx); i++ {
		m := bn.G2.MulScalar(setup.G2T[i], zx[i])
		g2Zt = bn.G2.Add(g2Zt, m)
	}
	proof.Vz = g2Zt
	proof.Va = bn.G2.MulScalar(bn.G2.G, setup.Toxic.Ka)
	proof.Vb = bn.G1.MulScalar(bn.G1.G, setup.Toxic.Kb)
	proof.Vc = bn.G2.MulScalar(bn.G2.G, setup.Toxic.Kc)

	return proof, nil
}

func VerifyProof(bn bn128.Bn128, setup Setup, proof Proof) bool {

	// e(piA, Va) == e(piA', g2)
	pairingPiaVa, err := bn.Pairing(proof.PiA, proof.Va)
	if err != nil {
		return false
	}
	pairingPiapG2, err := bn.Pairing(proof.PiAp, bn.G2.G)
	if err != nil {
		return false
	}
	if !bn.Fq12.Equal(pairingPiaVa, pairingPiapG2) {
		return false
	}

	// e(Vb, piB) == e(piB', g2)
	pairingVbPib, err := bn.Pairing(proof.Vb, proof.PiB)
	if err != nil {
		return false
	}
	pairingPibpG2, err := bn.Pairing(proof.PiBp, bn.G2.G)
	if err != nil {
		return false
	}
	if !bn.Fq12.Equal(pairingVbPib, pairingPibpG2) {
		return false
	}

	// e(piC, Vc) == e(piC', g2)
	pairingPicVc, err := bn.Pairing(proof.PiC, proof.Vc)
	if err != nil {
		return false
	}
	pairingPicpG2, err := bn.Pairing(proof.PiCp, bn.G2.G)
	if err != nil {
		return false
	}
	if !bn.Fq12.Equal(pairingPicVc, pairingPicpG2) {
		return false
	}

	// e(piA, piB) == e(piH, Vz) * e(piC, g2)
	// pairingPiaPib, err := bn.Pairing(proof.PiA, proof.PiB)
	// if err != nil {
	//         return false
	// }
	// pairingPihVz, err := bn.Pairing(proof.PiH, proof.Vz)
	// if err != nil {
	//         return false
	// }
	// pairingPicG2, err := bn.Pairing(proof.PiC, bn.G2.G)
	// if err != nil {
	//         return false
	// }
	// if !bn.Fq12.Equal(pairingPiaPib, bn.Fq12.Mul(pairingPihVz, pairingPicG2)) {
	//         return false
	// }

	return true
}
