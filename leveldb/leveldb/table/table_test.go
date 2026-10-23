// Copyright (c) 2014, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package table

import (
	"bytes"
	"encoding/binary"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/hemilabs/x/leveldb/leveldb/iterator"
	"github.com/hemilabs/x/leveldb/leveldb/opt"
	"github.com/hemilabs/x/leveldb/leveldb/storage"
	"github.com/hemilabs/x/leveldb/leveldb/testutil"
	"github.com/hemilabs/x/leveldb/leveldb/util"
)

type tableWrapper struct {
	*Reader
}

func (t tableWrapper) TestFind(key []byte) (rkey, rvalue []byte, err error) {
	return t.Reader.Find(key, false, nil)
}

func (t tableWrapper) TestGet(key []byte) (value []byte, err error) {
	return t.Reader.Get(key, nil)
}

func (t tableWrapper) TestNewIterator(slice *util.Range) iterator.Iterator {
	return t.Reader.NewIterator(slice, nil)
}

var _ = testutil.Defer(func() {
	Describe("Table", func() {
		Describe("approximate offset test", func() {
			var (
				buf = &bytes.Buffer{}
				o   = &opt.Options{
					BlockSize:   1024,
					Compression: opt.NoCompression,
				}
			)

			// Building the table.
			tw := NewWriter(buf, o, nil, 0)
			err := tw.Append([]byte("k01"), []byte("hello"))
			Expect(err).ShouldNot(HaveOccurred())
			err = tw.Append([]byte("k02"), []byte("hello2"))
			Expect(err).ShouldNot(HaveOccurred())
			err = tw.Append([]byte("k03"), bytes.Repeat([]byte{'x'}, 10000))
			Expect(err).ShouldNot(HaveOccurred())
			err = tw.Append([]byte("k04"), bytes.Repeat([]byte{'x'}, 200000))
			Expect(err).ShouldNot(HaveOccurred())
			err = tw.Append([]byte("k05"), bytes.Repeat([]byte{'x'}, 300000))
			Expect(err).ShouldNot(HaveOccurred())
			err = tw.Append([]byte("k06"), []byte("hello3"))
			Expect(err).ShouldNot(HaveOccurred())
			err = tw.Append([]byte("k07"), bytes.Repeat([]byte{'x'}, 100000))
			Expect(err).ShouldNot(HaveOccurred())

			err = tw.Close()

			It("Should be able to approximate offset of a key correctly", func() {
				Expect(err).ShouldNot(HaveOccurred())

				tr, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), storage.FileDesc{}, nil, nil, o)
				Expect(err).ShouldNot(HaveOccurred())
				CheckOffset := func(key string, expect, threshold int) {
					offset, err := tr.OffsetOf([]byte(key))
					Expect(err).ShouldNot(HaveOccurred())
					Expect(offset).Should(BeNumerically("~", expect, threshold), "Offset of key %q", key)
				}

				CheckOffset("k0", 0, 0)
				CheckOffset("k01a", 0, 0)
				CheckOffset("k02", 0, 0)
				CheckOffset("k03", 0, 0)
				CheckOffset("k04", 10000, 1000)
				CheckOffset("k04a", 210000, 1000)
				CheckOffset("k05", 210000, 1000)
				CheckOffset("k06", 510000, 1000)
				CheckOffset("k07", 510000, 1000)
				CheckOffset("xyz", 610000, 2000)
			})
		})

		Describe("read test", func() {
			Build := func(kv testutil.KeyValue) testutil.DB {
				o := &opt.Options{
					BlockSize:            512,
					BlockRestartInterval: 3,
				}
				buf := &bytes.Buffer{}

				// Building the table.
				tw := NewWriter(buf, o, nil, 0)
				kv.Iterate(func(i int, key, value []byte) {
					Expect(tw.Append(key, value)).ShouldNot(HaveOccurred())
				})
				tw.Close()

				// Opening the table.
				tr, _ := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), storage.FileDesc{}, nil, nil, o)
				return tableWrapper{tr}
			}
			Test := func(kv *testutil.KeyValue, body func(r *Reader)) func() {
				return func() {
					db := Build(*kv)
					if body != nil {
						body(db.(tableWrapper).Reader)
					}
					testutil.KeyValueTesting(nil, *kv, db, nil, nil)
				}
			}

			testutil.AllKeyValueTesting(nil, Build, nil, nil)
			Describe("with one key per block", Test(testutil.KeyValue_Generate(nil, 9, 1, 1, 10, 512, 512), func(r *Reader) {
				It("should have correct blocks number", func() {
					indexBlock, err := r.readBlock(r.indexBH, true)
					Expect(err).To(BeNil())
					Expect(indexBlock.restartsLen).Should(Equal(9))
				})
			}))
		})
	})
})

// TestDecodeLengthsNoPanic verifies that an overflowed block handle, a
// block handle past the end of the file, a bad restart count and a bad
// entry give errors and not panics.
func TestDecodeLengthsNoPanic(t *testing.T) {
	overflow := bytes.Repeat([]byte{0xff}, 11)
	if _, n := decodeBlockHandle(overflow); n != 0 {
		t.Fatal("decodeBlockHandle accepted an overflowed varint")
	}

	// A raw block: payload, type byte 0 and a checksum we do not check.
	payload := []byte{0xff, 0xff, 0xff, 0x7f} // restart count 2^31-1
	file := append(append(payload, 0), 0, 0, 0, 0)
	r := &Reader{reader: bytes.NewReader(file), size: int64(len(file)), bpool: util.NewBufferPool(4096)}
	if _, err := r.readRawBlock(blockHandle{offset: 1, length: uint64(len(file))}, false); err == nil {
		t.Fatal("readRawBlock accepted a handle past the end of the file")
	}
	if _, err := r.readBlock(blockHandle{offset: 0, length: uint64(len(payload))}, false); err == nil {
		t.Fatal("readBlock accepted a bad restart count")
	}

	b := &block{data: append(bytes.Repeat([]byte{0xff}, 10), 0x01), restartsOffset: 11}
	if _, _, _, _, err := b.entry(0); err == nil {
		t.Fatal("entry accepted a bad varint")
	}
}

// TestDecodeBlockBoundsNoPanic covers the snappy length bound, a restart
// offset past the entries, and a shared prefix longer than the key.
func TestDecodeBlockBoundsNoPanic(t *testing.T) {
	newReader := func(file []byte) *Reader {
		return &Reader{reader: bytes.NewReader(file), size: int64(len(file)), bpool: util.NewBufferPool(4096)}
	}

	// Snappy block that claims a decoded length of 1 GiB.
	snappyBlock := binary.AppendUvarint(nil, 1<<30)
	file := append(append(append([]byte{}, snappyBlock...), blockTypeSnappyCompression), 0, 0, 0, 0)
	if _, err := newReader(file).readRawBlock(blockHandle{length: uint64(len(snappyBlock))}, false); err == nil {
		t.Fatal("readRawBlock accepted a snappy block that claims 1 GiB")
	}

	// One restart at offset 100 in a 9-byte block.
	payload := []byte{0, 1, 0, 'k', 100, 0, 0, 0, 1, 0, 0, 0}
	file = append(append(append([]byte{}, payload...), 0), 0, 0, 0, 0)
	if _, err := newReader(file).readBlock(blockHandle{length: uint64(len(payload))}, false); err == nil {
		t.Fatal("readBlock accepted a restart offset past the entries")
	}

	// The first entry shares 5 bytes with an empty previous key.
	payload = []byte{5, 1, 0, 'k', 0, 0, 0, 0, 1, 0, 0, 0}
	file = append(append(append([]byte{}, payload...), 0), 0, 0, 0, 0)
	r := newReader(file)
	b, err := r.readBlock(blockHandle{length: uint64(len(payload))}, false)
	if err != nil {
		t.Fatal(err)
	}
	it := r.newBlockIter(b, nil, nil, false)
	if it.First() || it.Error() == nil {
		t.Fatal("the iterator accepted a shared prefix longer than the key")
	}
}
