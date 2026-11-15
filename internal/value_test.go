package kv

import (
	"testing"
	"unsafe"
)

func TestValueSize(t *testing.T) {
	t.Logf("Sizeof Value %d bytes", unsafe.Sizeof(Value{}))
}
