package glt

import "github.com/veandco/go-sdl2/sdl"

var _ ElementWidget = &Padding{}
var _ Element = &PaddingElement{}
var _ HasChild = &Padding{}
var _ HasChildElement = &PaddingElement{}

type EdgeInsets struct {
	Left   uint16
	Top    uint16
	Right  uint16
	Bottom uint16
}

func (c EdgeInsets) horizontal() uint16 {
	return c.Left + c.Right
}

func (c EdgeInsets) vertical() uint16 {
	return c.Top + c.Bottom
}
func EdgeInsetsAll(all uint16) EdgeInsets {
	return EdgeInsets{all, all, all, all}
}

func EdgeInsetsSymmetric(vertical, horizontal uint16) EdgeInsets {
	return EdgeInsets{horizontal, vertical, horizontal, vertical}
}

type Padding struct {
	Padding EdgeInsets
	Child   Widget
}

func (p *Padding) createElement() Element {
	pe := &PaddingElement{}
	pe.widget = p
	return pe
}

type PaddingElement struct {
	elementData
	childElementData
}

func (element *PaddingElement) layout(c Constraints) error {

	if element.child == nil {
		panic("padding with no child")
	}

	widget := element.widget.(*Padding)

	innerConstraints := c.deflate(widget.Padding)
	element.child.layout(innerConstraints)

	childSize := element.child.getSize()
	element.size = c.constrain(childSize.addMargin(widget.Padding))

	element.child.setOffset(Offset{widget.Padding.Left, widget.Padding.Top})

	return nil
}

func (element *PaddingElement) render(offset Offset, renderer *sdl.Renderer) {
	internalOffset := element.child.getOffset()
	offset.x += internalOffset.x
	offset.y += internalOffset.y
	element.child.render(offset, renderer)
}
