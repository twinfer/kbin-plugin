// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package eof_exception_sized

import (
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"bytes"
)

type EofExceptionSized struct {
	Buf *EofExceptionSized_Foo
	_io *kaitai.Stream
	_root *EofExceptionSized
	_parent interface{}
	_raw_Buf []byte
}
func NewEofExceptionSized() *EofExceptionSized {
	return &EofExceptionSized{
	}
}

func (this *EofExceptionSized) Read(io *kaitai.Stream, parent interface{}, root *EofExceptionSized) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1, err := this._io.ReadBytes(int(13))
	if err != nil {
		return err
	}
	tmp1 = tmp1
	this._raw_Buf = tmp1
	_io__raw_Buf := kaitai.NewStream(bytes.NewReader(this._raw_Buf))
	tmp2 := NewEofExceptionSized_Foo()
	err = tmp2.Read(_io__raw_Buf, this, this._root)
	if err != nil {
		return err
	}
	this.Buf = tmp2
	return err
}
type EofExceptionSized_Foo struct {
	_io *kaitai.Stream
	_root *EofExceptionSized
	_parent *EofExceptionSized
}
func NewEofExceptionSized_Foo() *EofExceptionSized_Foo {
	return &EofExceptionSized_Foo{
	}
}

func (this *EofExceptionSized_Foo) Read(io *kaitai.Stream, parent *EofExceptionSized, root *EofExceptionSized) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	return err
}
