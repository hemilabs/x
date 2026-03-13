// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package signing

import (
	"github.com/hemilabs/x/tss-lib/v2/common"
	"github.com/hemilabs/x/tss-lib/v2/ecdsa/keygen"
	"github.com/hemilabs/x/tss-lib/v2/tss"
)

// SigningState holds all mutable state between signing rounds.
// Opaque to the caller.
type SigningState struct {
	params *tss.Parameters
	key    *keygen.LocalPartySaveData
	data   *common.SignatureData
	temp   *localTempData
}

// SignRoundOutput holds the outbound messages and artifacts
// produced by a single signing round function.
type SignRoundOutput struct {
	// Messages to send.  Broadcast: GetTo() == nil.
	Messages []tss.Message

	// Signature is non-nil only after Finalize.
	Signature *common.SignatureData
}
