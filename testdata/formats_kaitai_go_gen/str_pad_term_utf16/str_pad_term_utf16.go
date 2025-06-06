// Code generated by kaitai-struct-compiler from a .ksy source file. DO NOT EDIT.

package str_pad_term_utf16

import (
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"golang.org/x/text/encoding/unicode"
)

type StrPadTermUtf16 struct {
	StrTerm string
	StrTermInclude string
	StrTermAndPad string
	_io *kaitai.Stream
	_root *StrPadTermUtf16
	_parent interface{}
}
func NewStrPadTermUtf16() *StrPadTermUtf16 {
	return &StrPadTermUtf16{
	}
}

func (this *StrPadTermUtf16) Read(io *kaitai.Stream, parent interface{}, root *StrPadTermUtf16) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1, err := this._io.ReadBytes(int(10))
	if err != nil {
		return err
	}
	tmp1 = kaitai.BytesTerminate(tmp1, 0, false)
	tmp2, err := kaitai.BytesToStr(tmp1, unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder())
	if err != nil {
		return err
	}
	this.StrTerm = tmp2
	tmp3, err := this._io.ReadBytes(int(10))
	if err != nil {
		return err
	}
	tmp3 = kaitai.BytesTerminate(tmp3, 0, true)
	tmp4, err := kaitai.BytesToStr(tmp3, unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder())
	if err != nil {
		return err
	}
	this.StrTermInclude = tmp4
	tmp5, err := this._io.ReadBytes(int(9))
	if err != nil {
		return err
	}
	tmp5 = kaitai.BytesTerminate(kaitai.BytesStripRight(tmp5, 43), 0, false)
	tmp6, err := kaitai.BytesToStr(tmp5, unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder())
	if err != nil {
		return err
	}
	this.StrTermAndPad = tmp6
	return err
}
