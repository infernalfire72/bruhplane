package main

import (
	"reflect"
	"unsafe"
)

type Stream struct {
	Data []byte
	Pos int
}

func (s *Stream)Extend(size int) {
	s.Data = append(s.Data[:s.Pos], append(make([]byte, size), s.Data[s.Pos:]...)...)
}

func (s *Stream)WriteByte(v byte) {
	s.Data = append(s.Data, v)
	s.Pos++
}

func (s *Stream)WriteInt64(v int64) {
	s.Extend(8)
	*(*int64)(unsafe.Pointer(&s.Data[s.Pos])) = v
	s.Pos += 8
}

func (s *Stream)WriteFloat64(v float64) {
	s.Extend(8)
	*(*float64)(unsafe.Pointer(&s.Data[s.Pos])) = v
	s.Pos += 8
}

func (s *Stream)WriteFloat32(v float32) {
	s.Extend(4)
	*(*float32)(unsafe.Pointer(&s.Data[s.Pos])) = v
	s.Pos += 4
}

func (s *Stream)WriteInt32(v int32) {
	s.Extend(4)
	*(*int32)(unsafe.Pointer(&s.Data[s.Pos])) = v
	s.Pos += 4
}

func (s *Stream)WriteInt16(v int16) {
	s.Extend(2)
	*(*int16)(unsafe.Pointer(&s.Data[s.Pos])) = v
	s.Pos += 2
}

func (s *Stream) ReadByte() byte {
	b := s.Data[s.Pos]
	s.Pos++
	return b
}

func ReadString(data []byte, pos *int) string {
	if data[*pos] != 11 {
		*pos++
		return ""
	}
	i := *pos + 1
	l := 0
	for {
		if data[i] == 128 {
			l += 127
			i++
		} else {
			l += int(data[i])
			i++
			break
		}
	}
	*pos = i
	s := string(data[i:i+l])
	*pos += l
	return s
}

func ReadStringNPos(v []byte) string {
	i := 0
	return ReadString(v, &i)
}

func (s *Stream)ReadString() string {
	return ReadString(s.Data, &s.Pos)
}

func (s *Stream)WriteString(v string) {
	if v == "" {
		s.Data = append(s.Data, 0)
		s.Pos++
		return
	}
	f := []byte(v)
	flen := len(f)
	s.Extend(flen + int(flen / 128) + 2)
	s.Data[s.Pos] = 11
	s.Pos++
	for {
		flen -= 127
		if flen < 1 {
			break
		}
		s.Data[s.Pos] = 128
		s.Pos++
	}
	s.Data[s.Pos] = byte(flen + 127)
	s.Pos++
	copy(s.Data[s.Pos:], f)
	s.Pos += len(f)
}

func (s *Stream)ReadIntArray() []int32 {
	l := *(*int16)(unsafe.Pointer(&s.Data[s.Pos]))
	s.Pos += 2
	slice := make([]int32, l)
	for i := int16(0); i < l; i++ {
		slice[i] = *(*int32)(unsafe.Pointer(&s.Data[s.Pos]))
		s.Pos += 4
	}
	return slice
}

func (s *Stream)WriteIntArray(v []int32) {
	s.Extend(len(v) * 4 + 2)
	*(*int16)(unsafe.Pointer(&s.Data[s.Pos])) = int16(len(v))
	s.Pos += 2
	for i := 0; i < len(v); i++ {
		*(*int32)(unsafe.Pointer(&s.Data[s.Pos])) = v[i]
		s.Pos += 4
	}
}

func (s *Stream)WriteByteArray(v []byte) {
	s.Extend(len(v))
	copy(s.Data[s.Pos:], v)
	s.Pos += len(v)
}

func (s *Stream) ReadInt16() int16 {
	c := (*int16)(unsafe.Pointer(&s.Data[s.Pos]))
	s.Pos += 2
	return *c
}

func (s *Stream) ReadInt32() int32 {
	c := (*int32)(unsafe.Pointer(&s.Data[s.Pos]))
	s.Pos += 4
	return *c
}

type Packetstream struct {
	PType int16
	Data Stream
	Items map[int]interface{}
}

func (p *Packetstream) AddData(v interface{}) {
	p.Items[len(p.Items)] = v
}

func (p *Packetstream) AddRange(v ...interface{}) {
	for i := 0; i < len(v); i++ {
		p.AddData(v[i])
	}
}

func (p *Packetstream) WritePacket(id int16, v ...interface{}) {
	p.Data.WriteInt16(id)
	p.Data.Extend(5)
	p.Data.Pos += 5
	pos := p.Data.Pos
	for i := 0; i < len(v); i++ {
		switch a := v[i].(type) {
			case int32:
				p.Data.WriteInt32(a)
			case string:
				p.Data.WriteString(a)
			case byte:
				p.Data.Data = append(p.Data.Data, a)
				p.Data.Pos++
			case int16:
				p.Data.WriteInt16(a)
			case int64:
				p.Data.WriteInt64(a)
			case float32:
				p.Data.WriteFloat32(a)
			case []byte:
				p.Data.WriteByteArray(a)
			case []int32:
				p.Data.WriteIntArray(a)
		}
	}
	*(*int32)(unsafe.Pointer(&p.Data.Data[pos-4])) = int32(p.Data.Pos-pos)
}

// Kept for compatibility
func (p *Packetstream) WriteData() {
	p.Data.WriteInt16(p.PType)
	p.Data.Extend(5)
	p.Data.Pos += 5
	pos := p.Data.Pos
	for i := 0; i < len(p.Items); i++ {
		t := reflect.TypeOf(p.Items[i])
		switch t.Name() {
		case "int32":
			p.Data.WriteInt32(p.Items[i].(int32))
			break
		case "string":
			p.Data.WriteString(p.Items[i].(string))
			break
		case "uint8":
			p.Data.Data = append(p.Data.Data, p.Items[i].(byte))
			p.Data.Pos++
			break
		case "int16":
			p.Data.WriteInt16(p.Items[i].(int16))
			break
		case "int64":
			p.Data.WriteInt64(p.Items[i].(int64))
			break
		case "float32":
			p.Data.WriteFloat32(p.Items[i].(float32))
			break
		case "":
			if t.Kind() != 23 {
				continue
			}

			switch t.Elem().Name() {
			case "uint8":
				p.Data.WriteByteArray(p.Items[i].([]byte))
				break
			case "int32":
				p.Data.WriteIntArray(p.Items[i].([]int32))
				break
			}
			break
		}
	}
	*(*int32)(unsafe.Pointer(&p.Data.Data[pos-4])) = int32(p.Data.Pos-pos)
	p.Items = make(map[int]interface{})
}