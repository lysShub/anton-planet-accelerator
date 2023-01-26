package fudp

import (
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/reedsolomon"
)

// fec layer

/*
	fec package structure:
	{origin-data : block-size(1B, /8) parity-length(1B) block-length(7b) group-header(1b)}
*/

const fecHdrSize = 3

type fec struct {
	// fec config, can be changed at runtime
	dataBlocks   int    // number of data blocks in a group
	parityBlocks int    // number of parity in a group
	blocks       int    // number of blocks in a group: dataBlocks + parityBlocks
	blockSize    int    // one block origin data size, in bytes
	_header      []byte // header of a fec package, with first-of-group flag false

	blockIdx int      // index of current block in group
	parities [][]byte // parity blocks in a group

	enc reedsolomon.Encoder

	m        *sync.RWMutex
	updateFn func() error

	startGroupNotify *sync.Cond
}

func NewFec(dataBlocks, parityBlocks, blockSize int) *fec {
	return &fec{}
}

// Wrap  encode data for FEC
//
//	in: origin data to encode
//	out: encoded data package
//	ok: indicate if in be encoded, if false, should Wrap again next
//	err: error
func (f *fec) Wrap(in []byte) (out []byte, ok bool, err error) {
	if len(in) == 0 {
		return nil, true, nil
	} else if len(in) > f.blockSize {
		return nil, false, errors.New("block too large")
	}
	f.m.Lock()
	defer func() { f.m.Unlock() }()

	if f.updateFn != nil {
		if err = f.updateFn(); err != nil {
			return nil, false, err
		}
	}

	if f.blockIdx >= f.dataBlocks {
		// finalize the group
		parityIdx := f.dataBlocks - f.blockIdx
		if parityIdx >= len(f.parities) {
			panic("")
		}
		out = f.parities[parityIdx]
		ok = false
		// TODO: optimize
		f.parities[parityIdx] = make([]byte, f.blockSize, f.blockSize+fecHdrSize)

	} else {
		// collect the group
		err = f.enc.EncodeIdx(in, f.blockIdx, f.parities)
		if err != nil {
			return nil, false, err
		}
		out = in
		ok = true
	}

	out = append(f._header, out...)
	if f.blockIdx == 0 {
		i := len(out) - 1
		out[i] = out[i] | 0b1
	}

	f.blockIdx = (f.blockIdx + 1) % f.blocks
	if f.blockIdx == 0 {
		f.startGroupNotify.Broadcast()
	}
	return out, ok, nil
}

// SetParity set ability of forward error correction.
// the function can called at the start of a group
func (f *fec) SetParity(dataBlocks, parityBlocks int) (<-chan struct{}, error) {
	if dataBlocks <= 0 || 0b1111111 < dataBlocks {
		return nil, errors.New("dataBlocks must be in range (0, 127)")
	} else if parityBlocks <= 0 || 255 < dataBlocks {
		return nil, fmt.Errorf("parityBlocks must be in range (0, 255)")
	}
	f.m.Lock()
	defer f.m.Unlock()

	if f.updateFn != nil {
		return nil, errors.New("can't set parity when the previous updating")
	}

	var notify = make(chan struct{})
	f.updateFn = func() error {
		defer func() {
			f.updateFn = nil
			close(notify)
		}()

		f.dataBlocks = dataBlocks
		f.parityBlocks = parityBlocks
		f.blocks = dataBlocks + parityBlocks
		return f.updateHdr()
	}

	return notify, nil
}

// SetBlockSize set size of one block.
// the function can called at the start of a group
func (f *fec) SetBlockSize(blockSize int) (<-chan struct{}, error) {
	if blockSize%8 != 0 {
		return nil, errors.New("block size must be multiple of 8")
	} else if blockSize <= 0 || blockSize > 1500 {
		return nil, errors.New("block size must be in range (0, 1500)")
	}
	f.m.Lock()
	defer f.m.Unlock()

	if f.updateFn != nil {
		return nil, errors.New("can't set block size when the previous updating")
	}

	var notify = make(chan struct{})
	f.updateFn = func() error {
		defer func() {
			f.updateFn = nil
			close(notify)
		}()
		f.blockSize = blockSize
		return f.updateHdr()
	}

	return notify, nil
}

func (f *fec) updateHdr() error {
	f._header = []byte{byte(f.blockSize / 8), byte(f.parityBlocks), byte(f.dataBlocks << 1)}

	if len(f.parities) != f.parityBlocks {
		//  update Parity
		f.parities = nil
		for i := 0; i < f.parityBlocks; i++ {
			f.parities = append(f.parities, make([]byte, f.blockSize, f.blockSize+fecHdrSize))
		}

		var err error
		f.enc, err = reedsolomon.New(f.dataBlocks, f.parityBlocks)
		if err != nil {
			return err
		}
	}

	if cap(f.parities[0]) != f.blockSize {
		// update blockSize
		f.parities = nil
		for i := 0; i < f.parityBlocks; i++ {
			f.parities = append(f.parities, make([]byte, f.blockSize, f.blockSize+fecHdrSize))
		}
	}

	return nil
}
