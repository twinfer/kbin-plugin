// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package eof_exception_bytes

import "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"

type EofExceptionBytes struct {
	Buf []byte
	_io *kaitai.Stream
	_root *EofExceptionBytes
	_parent interface{}
}
func NewEofExceptionBytes() *EofExceptionBytes {
	return &EofExceptionBytes{
	}
}

func (this *EofExceptionBytes) Read(io *kaitai.Stream, parent interface{}, root *EofExceptionBytes) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1, err := this._io.ReadBytes(int(13))
	if err != nil {
		return err
	}
	tmp1 = tmp1
	this.Buf = tmp1
	return err
}
