// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package term_strz

import "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"

type TermStrz struct {
	S1 string
	S2 string
	S3 string
	_io *kaitai.Stream
	_root *TermStrz
	_parent interface{}
}
func NewTermStrz() *TermStrz {
	return &TermStrz{
	}
}

func (this *TermStrz) Read(io *kaitai.Stream, parent interface{}, root *TermStrz) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1, err := this._io.ReadBytesTerm(124, false, true, true)
	if err != nil {
		return err
	}
	this.S1 = string(tmp1)
	tmp2, err := this._io.ReadBytesTerm(124, false, false, true)
	if err != nil {
		return err
	}
	this.S2 = string(tmp2)
	tmp3, err := this._io.ReadBytesTerm(64, true, true, true)
	if err != nil {
		return err
	}
	this.S3 = string(tmp3)
	return err
}
