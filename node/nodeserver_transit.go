package main

import (
	"errors"

	"github.com/gelembjuk/democoin/lib"
)

type NodeTransit struct {
	Blocks        map[string][][]byte
	MaxKnownHeigh int
	Logger        *lib.LoggerMan
}

func (t *NodeTransit) Init(l *lib.LoggerMan) error {
	t.Logger = l
	t.Blocks = make(map[string][][]byte)

	return nil
}
func (t *NodeTransit) AddBlocks(fromaddr lib.NodeAddr, blocks [][]byte) error {
	key := fromaddr.NodeAddrToString()

	_, ok := t.Blocks[key]

	if !ok {
		t.Blocks[key] = blocks
	} else {
		t.Blocks[key] = append(t.Blocks[key], blocks...)
	}

	return nil
}

func (t *NodeTransit) CleanBlocks(fromaddr lib.NodeAddr) {
	key := fromaddr.NodeAddrToString()

	if _, ok := t.Blocks[key]; ok {
		delete(t.Blocks, key)
	}
}

func (t *NodeTransit) GetBlocksCount(fromaddr lib.NodeAddr) int {
	if _, ok := t.Blocks[fromaddr.NodeAddrToString()]; ok {
		return len(t.Blocks[fromaddr.NodeAddrToString()])
	}
	return 0
}

func (t *NodeTransit) ShiftNextBlock(fromaddr lib.NodeAddr) ([]byte, error) {
	key := fromaddr.NodeAddrToString()

	if _, ok := t.Blocks[key]; ok {
		data := t.Blocks[key][0][:]
		t.Blocks[key] = t.Blocks[key][1:]

		if len(t.Blocks[key]) == 0 {
			delete(t.Blocks, key)
		}

		return data, nil
	}

	return nil, errors.New("The address is not in blocks transit")
}
