package dbg

import (
	"korok.io/korok/gfx/bk"
	"korok.io/korok/math/f32"

	"unsafe"
	"log"
	"fmt"
)

type DrawType uint8
const (
	TEXT DrawType = iota
	RECT
	CIRCLE
)

const step = 14
var screen struct{
	w, h float32
}
// max value of z-order
const zOrder = int32(0xFFFF>>1)

// dbg - draw debug info
// provide self-contained, im-gui api
// can be used to show debug info, fps..
// dependents: bk-api

var p2t2c4 = []bk.VertexComp{
	{4, bk.AttrFloat, 0, 0},
	{4, bk.AttrUInt8, 16, 1},
}

type PosTexColorVertex struct {
	X, Y, U, V float32
	RGBA       uint32
}

func Init(w, h int) {
	if gRender == nil {
		gRender = NewDebugRender(vsh, fsh)
		gBuffer = &gRender.Buffer
	}
	screen.w = float32(w)
	screen.h = float32(h)

	log.Println("dbg init w,h", w, h)
}

func SetScreenSize(w, h float32) {
	screen.w, screen.h = w, h
	gRender.SetViewPort(w, h)

	log.Println("set dbg size", w, h)
}

func Destroy() {
	if gBuffer != nil {
		gBuffer.Destroy()
	}
}

func FPS(fps int32) {
	x, y := gBuffer.x, gBuffer.y
	color := gBuffer.color

	gBuffer.x, gBuffer.y = 10, 10
	gBuffer.color = 0xFF121212
	DrawStr(fmt.Sprintf("%d FPS", fps))

	gBuffer.x, gBuffer.y = x, y
	gBuffer.color = color
}

func Move(x, y float32) {
	gBuffer.x, gBuffer.y = x, y
}

func Return() {
	gBuffer.y -= step
}

func Color(argb uint32) {
	gBuffer.color = argb
}

// draw a rect
func DrawRect(x, y, w, h float32) {
	gBuffer.Rect(x, y, w, h)
}

func DrawBorder(x, y, w, h, thickness float32) {
	gBuffer.Border(x, y, w, h, thickness)
}

// draw a circle
func DrawCircle(x,y float32, r float32) {

}

// draw string
func DrawStr(str string) {
	gBuffer.String(str, 1)
}

func DrawStrScaled(str string, scale float32) {
	gBuffer.String(str, scale)
}

func NextFrame() {
	gBuffer.Update()
	gRender.Draw()
	gBuffer.Reset()

}


type DebugRender struct {
	stateFlags uint64
	rgba       uint32

	// shader program
	program uint16

	// uniform handle
	umhProjection uint16 // Projection
	umhSampler0   uint16 // Sampler0

	// buffer
	Buffer TextShapeBuffer
}

func NewDebugRender(vsh, fsh string) *DebugRender {
	dr := new(DebugRender)
	// blend func
	dr.stateFlags |= bk.ST_BLEND.ALPHA_NON_PREMULTIPLIED

	// setup shader
	if id, sh := bk.R.AllocShader(vsh, fsh); id != bk.InvalidId {
		dr.program = id
		sh.Use()

		// setup attribute
		sh.AddAttributeBinding("xyuv\x00", 0, p2t2c4[0])
		sh.AddAttributeBinding("rgba\x00", 0, p2t2c4[1])

		s0 := int32(0)
		// setup uniform
		if pid, _ := bk.R.AllocUniform(id, "projection\x00", bk.UniformMat4, 1); pid != bk.InvalidId {
			dr.umhProjection = pid
		}
		if sid,_ := bk.R.AllocUniform(id, "tex\x00", bk.UniformSampler, 1); sid != bk.InvalidId {
			dr.umhSampler0 = sid
			bk.SetUniform(sid, unsafe.Pointer(&s0))
		}

		// submit render state
		//bk.Touch(0)
		bk.Submit(0, id, zOrder)
	}
	// setup buffer, we can draw 512 rect at most!!
	dr.Buffer.init(2048)
	return dr
}

func (dr *DebugRender) SetViewPort(w, h float32) {
	p := f32.Ortho2D(0, w, 0, h)
	bk.SetUniform(dr.umhProjection, unsafe.Pointer(&p[0]))
	bk.Submit(0, dr.program, zOrder)
}

func (dr *DebugRender) Draw() {
	bk.SetState(dr.stateFlags, dr.rgba)
	bk.SetTexture(0, dr.umhSampler0, uint16(dr.Buffer.fontTexId), 0)

	b := &dr.Buffer
	// set vertex
	bk.SetVertexBuffer(0, b.vertexId, 0, b.pos)
	bk.SetIndexBuffer(dr.Buffer.indexId, 0, b.pos * 6 >> 2)
	// submit
	bk.Submit(0, dr.program, zOrder)
}

type TextShapeBuffer struct {
	// real data
	vertex []PosTexColorVertex
	index  []uint16

	// gpu res
	indexId, vertexId uint16
	ib *bk.IndexBuffer
	vb *bk.VertexBuffer
	fontTexId uint16

	// current cursor position and painter color
	x, y float32
	color uint32

	// current buffer position
	pos uint32
}

func (buff *TextShapeBuffer) init(maxVertex uint32) {
	iboSize := maxVertex * 6 / 4
	buff.index = make([]uint16, iboSize)
	iFormat := [6]uint16 {3, 1, 2, 3, 2, 0}
	for i := uint32(0); i < iboSize; i += 6 {
		copy(buff.index[i:], iFormat[:])
		iFormat[0] += 4
		iFormat[1] += 4
		iFormat[2] += 4
		iFormat[3] += 4
		iFormat[4] += 4
		iFormat[5] += 4
	}
	if id, ib := bk.R.AllocIndexBuffer(bk.Memory{unsafe.Pointer(&buff.index[0]), iboSize}); id != bk.InvalidId {
		buff.indexId = id
		buff.ib = ib
	}

	buff.vertex = make([]PosTexColorVertex, maxVertex)
	vboSize := maxVertex * 20
	if id, vb := bk.R.AllocVertexBuffer(bk.Memory{nil, vboSize}, 20); id != bk.InvalidId {
		buff.vertexId = id
		buff.vb = vb
	}

	// texture
	img, fmt, err := LoadFontImage()
	if err != nil {
		log.Println("fail to load font image.. fmt:", fmt)
	}
	if id, _ := bk.R.AllocTexture(img); id != bk.InvalidId {
		buff.fontTexId = id
	}
}

//
//  3-------0
//  |       |
//  |       |
//  1-------2
func (buff *TextShapeBuffer) String(chars string, scale float32) {
	x, y := float32(0), float32(0)
	w, h := font_width * scale, font_height * scale

	for i, N := 0, len(chars); i < N; i++ {
		b := buff.vertex[buff.pos: buff.pos+4]
		buff.pos += 4

		// vv := chars[0]
		var left, right, bottom, top float32 = GlyphRegion(chars[i])
		bottom, top = top, bottom

		b[0].X, b[0].Y = buff.x + x + w, buff.y + y + h
		b[0].U, b[0].V = right, top
		b[0].RGBA = buff.color

		b[1].X, b[1].Y = buff.x + x + 0, buff.y + y + 0
		b[1].U, b[1].V = left, bottom
		b[1].RGBA = buff.color

		b[2].X, b[2].Y = buff.x + x + w, buff.y + y + 0
		b[2].U, b[2].V = right, bottom
		b[2].RGBA = buff.color

		b[3].X, b[3].Y = buff.x + x + 0, buff.y + y + h
		b[3].U, b[3].V = left, top
		b[3].RGBA = buff.color

		// advance x,y
		x += w
	}
}


//
//  3-------0
//  |       |
//  |       |
//  1-------2
func (buff *TextShapeBuffer) Rect(x,y, w, h float32) {
	b := buff.vertex[buff.pos: buff.pos+4]
	buff.pos += 4

	b[0].X, b[0].Y = buff.x + x + w, buff.y + y + h
	b[0].U, b[0].V = 2, 0
	b[0].RGBA = buff.color

	b[1].X, b[1].Y = buff.x + x + 0, buff.y + y + 0
	b[1].U, b[1].V = 2, 0
	b[1].RGBA = buff.color

	b[2].X, b[2].Y = buff.x + x + w, buff.y + y + 0
	b[2].U, b[2].V = 2, 0
	b[2].RGBA = buff.color

	b[3].X, b[3].Y = buff.x + x + 0, buff.y + y + h
	b[3].U, b[3].V = 2, 0
	b[3].RGBA = buff.color
}

func (buff *TextShapeBuffer) Border(x, y, w, h, thick float32) {
	buff.Rect(x,y,w,thick)
	buff.Rect(x,y+h-thick,w,thick)
	buff.Rect(x, y, thick, h)
	buff.Rect(x+w-thick,y,thick,h)
}

func (buff *TextShapeBuffer) Update() {
	buff.vb.Update(0, buff.pos * 20, unsafe.Pointer(&buff.vertex[0]), false)
}

func (buff *TextShapeBuffer) Reset() {
	buff.pos = 0
	buff.x = 10
	buff.y = screen.h - step*1.5
}

func (buff *TextShapeBuffer) Destroy() {

}

//// static filed
var gRender *DebugRender
var gBuffer *TextShapeBuffer

