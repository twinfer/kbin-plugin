// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package valid_optional_id

import (
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"bytes"
)

type ValidOptionalId struct {
	_unnamed0 []byte
	_unnamed1 uint8
	_unnamed2 int8
	_io *kaitai.Stream
	_root *ValidOptionalId
	_parent interface{}
}
func NewValidOptionalId() *ValidOptionalId {
	return &ValidOptionalId{
	}
}

func (this *ValidOptionalId) Read(io *kaitai.Stream, parent interface{}, root *ValidOptionalId) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1, err := this._io.ReadBytes(int(6))
	if err != nil {
		return err
	}
	tmp1 = tmp1
	this._unnamed0 = tmp1
	if !(bytes.Equal(this._unnamed0, []uint8{80, 65, 67, 75, 45, 49})) {
		return kaitai.NewValidationNotEqualError([]uint8{80, 65, 67, 75, 45, 49}, this._unnamed0, this._io, "/seq/0")
	}
	tmp2, err := this._io.ReadU1()
	if err != nil {
		return err
	}
	this._unnamed1 = tmp2
	if !(this._unnamed1 == 255) {
		return kaitai.NewValidationNotEqualError(255, this._unnamed1, this._io, "/seq/1")
	}
	tmp3, err := this._io.ReadS1()
	if err != nil {
		return err
	}
	this._unnamed2 = tmp3
	{
		_it := this._unnamed2
		if !(_it == -1) {
			return kaitai.NewValidationExprError(this._unnamed2, this._io, "/seq/2")
		}
	}
	return err
}
