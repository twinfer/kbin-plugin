// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package str_eos

import "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"

type StrEos struct {
	Str string
	_io *kaitai.Stream
	_root *StrEos
	_parent interface{}
}
func NewStrEos() *StrEos {
	return &StrEos{
	}
}

func (this *StrEos) Read(io *kaitai.Stream, parent interface{}, root *StrEos) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1, err := this._io.ReadBytesFull()
	if err != nil {
		return err
	}
	tmp1 = tmp1
	this.Str = string(tmp1)
	return err
}
