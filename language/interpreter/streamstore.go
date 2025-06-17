package interpreter

import (
	"fmt"

	"github.com/andyp1xe1/vidlang/language/parser"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

const (
	EntryStream = iota
	EntryStreamList
)

// Stream represents a media stream in our DSL
type Stream struct {
	FFStream *ffmpeg.Stream
}

type StreamList []*Stream

type SplitNode struct {
	*ffmpeg.Node
}

func (n *SplitNode) split(c int) interface{} {
	return &Stream{
		FFStream: n.Get(fmt.Sprintf("%v", c)),
	}
}

type SplitList struct {
	list []*SplitNode
}

func (l *SplitList) split(c int) interface{} {
	res := make(StreamList, 0)
	for _, n := range l.list {
		s := n.split(c).(*Stream)
		res = append(res, s)
	}
	return res
}

type storeNode interface{ split(c int) interface{} }

type streamStore struct {
	splitNodes     map[parser.NodeIdent]storeNode
	canCopyStreams map[parser.NodeIdent]interface{}
	splitCounts    map[parser.NodeIdent]int
}

func newStreamStore() streamStore {
	return streamStore{
		splitNodes:     make(map[parser.NodeIdent]storeNode),
		canCopyStreams: make(map[parser.NodeIdent]interface{}),
		splitCounts:    make(map[parser.NodeIdent]int),
	}
}

func (s streamStore) getAuto(name parser.NodeIdent) (interface{}, bool, error) {
	stream, ok := s.canCopyStreams[name]
	if ok {
		return stream, true, nil
	}
	stream, err := s.getSplit(name)
	if err != nil {
		return nil, false, err
	}
	fmt.Printf("split count: %v\n", s.splitCounts[name])
	s.splitCounts[name] = s.splitCounts[name] + 1
	return stream, false, nil

}

func (s streamStore) getSplit(name parser.NodeIdent) (interface{}, error) {
	fnode, ok := s.splitNodes[name]
	if !ok {
		return nil, fmt.Errorf("stream variable %s not defined", name)
	}
	stream := fnode.split(s.splitCounts[name])
	s.splitCounts[name] = s.splitCounts[name] + 1
	return stream, nil
}

func (s streamStore) set(name parser.NodeIdent, entry interface{}, canCopy bool) {
	if canCopy {
		s.canCopyStreams[name] = entry
	}

	if stream, ok := entry.(*Stream); ok {
		s.splitNodes[name] = &SplitNode{stream.FFStream.Split()}

	} else if list, ok := entry.(StreamList); ok {

		spList := SplitList{make([]*SplitNode, 0)}

		for _, s := range []*Stream(list) {
			spList.list = append(spList.list, &SplitNode{s.FFStream.Split()})
		}

		s.splitNodes[name] = &spList
	}

	s.splitCounts[name] = 0
}
