// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/crypto"
	"github.com/hemilabs/x/tss-lib/v2/crypto/commitments"
	"github.com/hemilabs/x/tss-lib/v2/eddsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// round 1 represents round 1 of the signing part of the EDDSA TSS spec
func newRound1(params *tss.Parameters, key *keygen.LocalPartySaveData, data *common.SignatureData, temp *localTempData, out chan<- tss.Message, end chan<- *common.SignatureData) tss.Round {
	return &round1{
		&base{params, key, data, temp, out, end, make([]bool, len(params.Parties().IDs())), false, 1},
	}
}

func (round *round1) Start() *tss.Error {
	if round.started {
		return round.WrapError(errors.New("round already started"))
	}

	round.number = 1
	round.started = true
	round.resetOK()

	// [FORK] Validate key material before use: upstream did not check. A nil or zero Xi
	// would produce a zero Lagrange-interpolated wi, reducing the effective threshold
	// (the other t parties could sign without this party's contribution). A nil or
	// off-curve EDDSAPub would cause panics or invalid signature verification.
	if round.key.Xi == nil || round.key.Xi.Sign() == 0 {
		return round.WrapError(errors.New("invalid key data: Xi is nil or zero"))
	}
	if round.key.EDDSAPub == nil || !round.key.EDDSAPub.ValidateBasic() {
		return round.WrapError(errors.New("invalid key data: EDDSAPub is nil or not on curve"))
	}

	// [FORK] Use caller-supplied SSIDNonce instead of upstream's hardcoded 0 (SC#662).
	round.temp.ssidNonce = new(big.Int).SetUint64(uint64(round.Params().SSIDNonce()))
	var err error
	round.temp.ssid, err = round.getSSID()
	if err != nil {
		return round.WrapError(err)
	}
	// 1. select ri
	ri := common.GetRandomPositiveInt(round.Rand(), round.Params().EC().Params().N)

	// 2. make commitment
	pointRi := crypto.ScalarBaseMult(round.Params().EC(), ri)
	cmt := commitments.NewHashCommitment(round.Rand(), pointRi.X(), pointRi.Y())

	// 3. store r1 message pieces
	round.temp.ri = ri
	round.temp.pointRi = pointRi
	round.temp.deCommit = cmt.D

	i := round.PartyID().Index
	round.ok[i] = true

	// 4. broadcast commitment
	r1msg2 := NewSignRound1Message(round.PartyID(), cmt.C)
	round.temp.signRound1Messages[i] = r1msg2
	round.out <- r1msg2

	return nil
}

func (round *round1) Update() (bool, *tss.Error) {
	ret := true
	for j, msg := range round.temp.signRound1Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			ret = false
			continue
		}
		round.ok[j] = true
	}
	return ret, nil
}

func (round *round1) CanAccept(msg tss.ParsedMessage) bool {
	if _, ok := msg.Content().(*SignRound1Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

func (round *round1) NextRound() tss.Round {
	round.started = false
	return &round2{round}
}

// ----- //

// helper to call into PrepareForSigning()
func (round *round1) prepare() error {
	i := round.PartyID().Index

	xi := round.key.Xi
	ks := round.key.Ks

	// [FORK] Key count validation: upstream only checked t+1 > len(ks), not len(ks) == partyCount.
	// A mismatch between key count and party count would cause index-out-of-bounds panics.
	if len(ks) != round.PartyCount() {
		return fmt.Errorf("key count %d does not match party count %d", len(ks), round.PartyCount())
	}
	if round.Threshold()+1 > len(ks) {
		return fmt.Errorf("t+1=%d is not satisfied by the key count of %d", round.Threshold()+1, len(ks))
	}
	wi := PrepareForSigning(round.Params().EC(), i, len(ks), xi, ks)

	round.temp.wi = wi
	return nil
}
