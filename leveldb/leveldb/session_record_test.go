// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package leveldb

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func decodeEncode(v *sessionRecord) (res bool, err error) {
	b := new(bytes.Buffer)
	err = v.encode(b)
	if err != nil {
		return
	}
	v2 := &sessionRecord{}
	err = v.decode(b)
	if err != nil {
		return
	}
	b2 := new(bytes.Buffer)
	err = v2.encode(b2)
	if err != nil {
		return
	}
	return bytes.Equal(b.Bytes(), b2.Bytes()), nil
}

func TestSessionRecord_EncodeDecode(t *testing.T) {
	big := int64(1) << 50
	v := &sessionRecord{}
	i := int64(0)
	test := func() {
		res, err := decodeEncode(v)
		if err != nil {
			t.Fatalf("error when testing encode/decode sessionRecord: %v", err)
		}
		if !res {
			t.Error("encode/decode test failed at iteration:", i)
		}
	}

	for ; i < 4; i++ {
		test()
		v.addTable(3, big+300+i, big+400+i,
			makeInternalKey(nil, []byte("foo"), uint64(big+500+1), keyTypeVal),
			makeInternalKey(nil, []byte("zoo"), uint64(big+600+1), keyTypeDel))
		v.delTable(4, big+700+i)
		v.addCompPtr(int(i), makeInternalKey(nil, []byte("x"), uint64(big+900+1), keyTypeVal))
	}

	v.setComparer("foo")
	v.setJournalNum(big + 100)
	v.setPrevJournalNum(big + 99)
	v.setNextFileNum(big + 200)
	v.setSeqNum(uint64(big + 1000))
	test()
}

// TestSessionRecordDecodeBadLengths verifies that a huge byte length
// or level gives an error, without a huge allocation or a panic.
func TestSessionRecordDecodeBadLengths(t *testing.T) {
	for name, data := range map[string][]byte{
		"comparer length": binary.AppendUvarint(binary.AppendUvarint(nil, recComparer), 1<<62),
		"level":           binary.AppendUvarint(binary.AppendUvarint(nil, recAddTable), math.MaxUint64),
	} {
		rec := &sessionRecord{}
		if err := rec.decode(bytes.NewReader(data)); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}
