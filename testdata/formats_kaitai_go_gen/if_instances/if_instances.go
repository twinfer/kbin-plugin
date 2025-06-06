// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package if_instances

import (
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"io"
)

type IfInstances struct {
	_io *kaitai.Stream
	_root *IfInstances
	_parent interface{}
	_f_neverHappens bool
	neverHappens uint8
}
func NewIfInstances() *IfInstances {
	return &IfInstances{
	}
}

func (this *IfInstances) Read(io *kaitai.Stream, parent interface{}, root *IfInstances) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	return err
}
func (this *IfInstances) NeverHappens() (v uint8, err error) {
	if (this._f_neverHappens) {
		return this.neverHappens, nil
	}
	if (false) {
		_pos, err := this._io.Pos()
		if err != nil {
			return 0, err
		}
		_, err = this._io.Seek(int64(100500), io.SeekStart)
		if err != nil {
			return 0, err
		}
		tmp1, err := this._io.ReadU1()
		if err != nil {
			return 0, err
		}
		this.neverHappens = tmp1
		_, err = this._io.Seek(_pos, io.SeekStart)
		if err != nil {
			return 0, err
		}
		this._f_neverHappens = true
	}
	this._f_neverHappens = true
	return this.neverHappens, nil
}
