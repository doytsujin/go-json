package json

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/xerrors"
)

type Encoder struct {
	buf  []byte
	pool sync.Pool
}

type EncodeOp func(*Encoder, uintptr)

const (
	bufSize = 1024
)

var (
	encPool        sync.Pool
	cachedEncodeOp map[string]EncodeOp
)

func init() {
	encPool = sync.Pool{
		New: func() interface{} {
			return &Encoder{
				buf:  make([]byte, 0, bufSize),
				pool: encPool,
			}
		},
	}
	cachedEncodeOp = map[string]EncodeOp{}
}

func NewEncoder() *Encoder {
	enc := encPool.Get().(*Encoder)
	enc.Reset()
	return enc
}

func (e *Encoder) Release() {
	e.pool.Put(e)
}

func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

func (e *Encoder) EncodeInt(v int) {
	e.EncodeInt64(int64(v))
}

func (e *Encoder) EncodeInt8(v int8) {
	e.EncodeInt64(int64(v))
}

func (e *Encoder) EncodeInt16(v int16) {
	e.EncodeInt64(int64(v))
}

func (e *Encoder) EncodeInt32(v int32) {
	e.EncodeInt64(int64(v))
}

func (e *Encoder) EncodeInt64(v int64) {
	e.buf = strconv.AppendInt(e.buf, v, 10)
}

func (e *Encoder) EncodeUint(v uint) {
	e.EncodeUint64(uint64(v))
}

func (e *Encoder) EncodeUint8(v uint8) {
	e.EncodeUint64(uint64(v))
}

func (e *Encoder) EncodeUint16(v uint16) {
	e.EncodeUint64(uint64(v))
}

func (e *Encoder) EncodeUint32(v uint32) {
	e.EncodeUint64(uint64(v))
}

func (e *Encoder) EncodeUint64(v uint64) {
	e.buf = strconv.AppendUint(e.buf, v, 10)
}

func (e *Encoder) EncodeFloat32(v float32) {
	e.buf = strconv.AppendFloat(e.buf, float64(v), 'f', -1, 32)
}

func (e *Encoder) EncodeFloat64(v float64) {
	e.buf = strconv.AppendFloat(e.buf, v, 'f', -1, 64)
}

func (e *Encoder) EncodeBool(v bool) {
	e.buf = strconv.AppendBool(e.buf, v)
}

func (e *Encoder) EncodeString(s string) {
	b := *(*[]byte)(unsafe.Pointer(&s))
	e.buf = append(e.buf, b...)
}

func (e *Encoder) EncodeByte(b byte) {
	e.buf = append(e.buf, b)
}

type rtype struct{}

type interfaceHeader struct {
	typ *rtype
	ptr unsafe.Pointer
}

func (e *Encoder) Encode(v interface{}) ([]byte, error) {
	header := (*interfaceHeader)(unsafe.Pointer(&v))
	return e.encode(reflect.ValueOf(v), header.ptr)
}

func (e *Encoder) encode(v reflect.Value, ptr unsafe.Pointer) ([]byte, error) {
	typ := v.Type()
	name := typ.String()
	if op, exists := cachedEncodeOp[name]; exists {
		op(e, uintptr(ptr))
		copied := make([]byte, len(e.buf))
		copy(copied, e.buf)
		return copied, nil
	}
	if typ.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	op, err := e.compile(v)
	if err != nil {
		return nil, err
	}
	if op == nil {
		return nil, nil
	}
	if name != "" {
		cachedEncodeOp[name] = op
	}
	op(e, uintptr(ptr))
	copied := make([]byte, len(e.buf))
	copy(copied, e.buf)
	return copied, nil
}

func (e *Encoder) compile(v reflect.Value) (EncodeOp, error) {
	switch v.Type().Kind() {
	case reflect.Ptr:
		return e.compilePtr(v)
	case reflect.Slice:
		return e.compileSlice(v)
	case reflect.Struct:
		return e.compileStruct(v)
	case reflect.Map:
		return e.compileMap(v)
	case reflect.Array:
		return e.compileArray(v)
	case reflect.Int:
		return e.compileInt()
	case reflect.Int8:
		return e.compileInt8()
	case reflect.Int16:
		return e.compileInt16()
	case reflect.Int32:
		return e.compileInt32()
	case reflect.Int64:
		return e.compileInt64()
	case reflect.Uint:
		return e.compileUint()
	case reflect.Uint8:
		return e.compileUint8()
	case reflect.Uint16:
		return e.compileUint16()
	case reflect.Uint32:
		return e.compileUint32()
	case reflect.Uint64:
		return e.compileUint64()
	case reflect.Float32:
		return e.compileFloat32()
	case reflect.Float64:
		return e.compileFloat64()
	case reflect.String:
		return e.compileString()
	case reflect.Bool:
		return e.compileBool()
	case reflect.Interface:
		return nil, ErrCompileSlowPath
	case reflect.Func:
		return nil, nil
	case reflect.Chan:
		return nil, nil
	case reflect.UnsafePointer:
		return nil, nil
	case reflect.Uintptr:
		return nil, nil
	case reflect.Complex64:
		return nil, nil
	case reflect.Complex128:
		return nil, nil
	}
	return nil, xerrors.Errorf("failed to compile %s: %w", v.Type(), ErrUnknownType)
}

func (e *Encoder) compilePtr(v reflect.Value) (EncodeOp, error) {
	op, err := e.compile(v.Elem())
	if err != nil {
		return nil, err
	}
	if op == nil {
		return nil, nil
	}
	return func(enc *Encoder, p uintptr) {
		op(enc, e.ptrToPtr(p))
	}, nil
}

func (e *Encoder) compileInt() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeInt(e.ptrToInt(p)) }, nil
}

func (e *Encoder) compileInt8() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeInt8(e.ptrToInt8(p)) }, nil
}

func (e *Encoder) compileInt16() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeInt16(e.ptrToInt16(p)) }, nil
}

func (e *Encoder) compileInt32() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeInt32(e.ptrToInt32(p)) }, nil
}

func (e *Encoder) compileInt64() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeInt64(e.ptrToInt64(p)) }, nil
}

func (e *Encoder) compileUint() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeUint(e.ptrToUint(p)) }, nil
}

func (e *Encoder) compileUint8() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeUint8(e.ptrToUint8(p)) }, nil
}

func (e *Encoder) compileUint16() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeUint16(e.ptrToUint16(p)) }, nil
}

func (e *Encoder) compileUint32() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeUint32(e.ptrToUint32(p)) }, nil
}

func (e *Encoder) compileUint64() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeUint64(e.ptrToUint64(p)) }, nil
}

func (e *Encoder) compileFloat32() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeFloat32(e.ptrToFloat32(p)) }, nil
}

func (e *Encoder) compileFloat64() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeFloat64(e.ptrToFloat64(p)) }, nil
}

func (e *Encoder) compileString() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeEscapedString(e.ptrToString(p)) }, nil
}

func (e *Encoder) compileBool() (EncodeOp, error) {
	return func(enc *Encoder, p uintptr) { enc.EncodeBool(e.ptrToBool(p)) }, nil
}

func (e *Encoder) compileSlice(v reflect.Value) (EncodeOp, error) {
	typ := v.Type()
	size := typ.Elem().Size()
	op, err := e.compile(reflect.Zero(typ.Elem()))
	if err != nil {
		return nil, err
	}
	if op == nil {
		return nil, nil
	}
	return func(enc *Encoder, base uintptr) {
		if base == 0 {
			enc.EncodeString("null")
			return
		}
		enc.EncodeByte('[')
		slice := (*reflect.SliceHeader)(unsafe.Pointer(base))
		num := slice.Len
		for i := 0; i < num; i++ {
			op(enc, slice.Data+uintptr(i)*size)
			if i != num-1 {
				enc.EncodeByte(',')
			}
		}
		enc.EncodeByte(']')
	}, nil
}

func (e *Encoder) compileArray(v reflect.Value) (EncodeOp, error) {
	typ := v.Type()
	alen := typ.Len()
	size := typ.Elem().Size()
	op, err := e.compile(reflect.Zero(typ.Elem()))
	if err != nil {
		return nil, err
	}
	if op == nil {
		return nil, nil
	}
	return func(enc *Encoder, base uintptr) {
		if base == 0 {
			enc.EncodeString("null")
			return
		}
		enc.EncodeByte('[')
		for i := 0; i < alen; i++ {
			if i != 0 {
				enc.EncodeByte(',')
			}
			op(enc, base+uintptr(i)*size)
		}
		enc.EncodeByte(']')
	}, nil
}

func (e *Encoder) compileStruct(v reflect.Value) (EncodeOp, error) {
	typ := v.Type()
	fieldNum := typ.NumField()
	opQueue := make([]EncodeOp, 0, fieldNum)
	for i := 0; i < fieldNum; i++ {
		field := typ.Field(i)
		keyName := field.Name
		tag := field.Tag.Get("json")
		opts := strings.Split(tag, ",")
		if len(opts) > 0 {
			if opts[0] != "" {
				keyName = opts[0]
			}
		}
		op, err := e.compile(v.Field(i))
		if err != nil {
			return nil, err
		}
		if op == nil {
			continue
		}
		key := fmt.Sprintf(`"%s":`, keyName)
		opQueue = append(opQueue, func(enc *Encoder, base uintptr) {
			enc.EncodeString(key)
			op(enc, base+field.Offset)
		})
	}
	queueNum := len(opQueue)
	return func(enc *Encoder, base uintptr) {
		if base == 0 {
			enc.EncodeString("null")
			return
		}
		enc.EncodeByte('{')
		for i := 0; i < queueNum; i++ {
			opQueue[i](enc, base)
			if i != queueNum-1 {
				enc.EncodeByte(',')
			}
		}
		enc.EncodeByte('}')
	}, nil
}

//go:linkname mapiterinit reflect.mapiterinit
func mapiterinit(mapType unsafe.Pointer, m unsafe.Pointer) unsafe.Pointer

//go:linkname mapiterkey reflect.mapiterkey
func mapiterkey(it unsafe.Pointer) unsafe.Pointer

//go:linkname mapitervalue reflect.mapitervalue
func mapitervalue(it unsafe.Pointer) unsafe.Pointer

//go:linkname mapiternext reflect.mapiternext
func mapiternext(it unsafe.Pointer)

//go:linkname maplen reflect.maplen
func maplen(m unsafe.Pointer) int

type valueType struct {
	typ unsafe.Pointer
	ptr unsafe.Pointer
}

func (e *Encoder) compileMap(v reflect.Value) (EncodeOp, error) {
	mapType := (*valueType)(unsafe.Pointer(&v)).typ
	keyOp, err := e.compile(reflect.Zero(v.Type().Key()))
	if err != nil {
		return nil, err
	}
	if keyOp == nil {
		return nil, nil
	}
	valueOp, err := e.compile(reflect.Zero(v.Type().Elem()))
	if err != nil {
		return nil, err
	}
	if valueOp == nil {
		return nil, nil
	}
	return func(enc *Encoder, base uintptr) {
		if base == 0 {
			enc.EncodeString("null")
			return
		}
		enc.EncodeByte('{')
		mlen := maplen(unsafe.Pointer(base))
		iter := mapiterinit(mapType, unsafe.Pointer(base))
		for i := 0; i < mlen; i++ {
			key := mapiterkey(iter)
			if i != 0 {
				enc.EncodeByte(',')
			}
			value := mapitervalue(iter)
			keyOp(enc, uintptr(key))
			enc.EncodeByte(':')
			valueOp(enc, uintptr(value))
			mapiternext(iter)
		}
		enc.EncodeByte('}')
	}, nil
}

func (e *Encoder) ptrToPtr(p uintptr) uintptr     { return *(*uintptr)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToInt(p uintptr) int         { return *(*int)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToInt8(p uintptr) int8       { return *(*int8)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToInt16(p uintptr) int16     { return *(*int16)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToInt32(p uintptr) int32     { return *(*int32)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToInt64(p uintptr) int64     { return *(*int64)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToUint(p uintptr) uint       { return *(*uint)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToUint8(p uintptr) uint8     { return *(*uint8)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToUint16(p uintptr) uint16   { return *(*uint16)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToUint32(p uintptr) uint32   { return *(*uint32)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToUint64(p uintptr) uint64   { return *(*uint64)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToFloat32(p uintptr) float32 { return *(*float32)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToFloat64(p uintptr) float64 { return *(*float64)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToBool(p uintptr) bool       { return *(*bool)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToByte(p uintptr) byte       { return *(*byte)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToBytes(p uintptr) []byte    { return *(*[]byte)(unsafe.Pointer(p)) }
func (e *Encoder) ptrToString(p uintptr) string   { return *(*string)(unsafe.Pointer(p)) }
