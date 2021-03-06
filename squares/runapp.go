package squares

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"runtime"

	"github.com/veandco/go-sdl2/gfx"
	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

const DEFAULT_WINDOW_WIDTH = 380
const DEFAULT_WINDOW_HEIGHT = 600

var renderer *sdl.Renderer
var font *ttf.Font

func initRender(title string) {

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS | sdl.INIT_TIMER); err != nil {
		panic(err)
	}
	// defer sdl.Quit()
	if flags := img.Init(img.INIT_PNG); flags != img.INIT_PNG {
		panic(img.GetError())
	}

	window, err := sdl.CreateWindow(title, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		DEFAULT_WINDOW_WIDTH, DEFAULT_WINDOW_HEIGHT, sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE)
	if err != nil {
		panic(err)
	}

	renderer, err = sdl.CreateRenderer(window, -1, 0)
	if err != nil {
		panic(err)
	}
	//defer window.Destroy()

	err = ttf.Init()
	if err != nil {
		panic(err)
	}
	_, file, _, _ := runtime.Caller(0)
	font, err = ttf.OpenFont(path.Dir(file)+"/fonts/OpenSans-Regular.ttf", 12)
	if err != nil {
		panic(err)
	}
}

func render(windowElement Element) {

	renderer.SetDrawColor(255, 255, 255, 255)
	renderer.Clear()

	windowElement.render(Offset{0, 0}, renderer)

	renderer.Present()
}

func findWidget(typeW Widget, searchW Widget) Widget {
	if searchW == nil {
		return nil
	} else if sameType(typeW, searchW) {
		return searchW
	} else if pw, ok := searchW.(HasChild); ok {
		return findWidget(typeW, pw.getChild())
	} else if pw, ok := searchW.(HasChildren); ok {
		for _, child := range pw.getChildren() {
			r := findWidget(typeW, child)
			if r != nil {
				return r
			}
		}
		return nil
	}
	return nil
}

func RunApp(app Widget) error {

	title := "NO TITLE"
	widget := findWidget(&MaterialApp{}, app)
	if widget != nil {
		widgetTitle := widget.(*MaterialApp).Title
		if widgetTitle != "" {
			title = widgetTitle
		}
	}

	initRender(title)
	initIcons()

	fps := &gfx.FPSmanager{}
	gfx.InitFramerate(fps)
	gfx.SetFramerate(fps, 60)

	windowWidget := Window{
		Child: app,
	}
	windowElement, err := buildElementTree(windowWidget, nil)
	if err != nil {
		return err
	}

	updateWindowSize := func(width, height float64) {
		windowElement.(*StatefulElement).SetState(func() {
			windowElement.(*StatefulElement).GetState().(*WindowState).Size = Size{
				Width:  float64(width),
				Height: float64(height),
			}
		})
	}

	updateWindowSize(DEFAULT_WINDOW_WIDTH, DEFAULT_WINDOW_HEIGHT)

	running := true
	for running {

		err = rebuildDirty(windowElement)
		if err != nil {
			return err
		}

		err = windowElement.layout(ConstraintsUnbounded())
		if err != nil {
			return err
		}

		floatUpRendered(windowElement)

		gfx.FramerateDelay(fps)

		render(windowElement)

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event := event.(type) {
			case *sdl.QuitEvent:
				running = false
				break
			case *sdl.KeyboardEvent:
				if event.Type == sdl.KEYDOWN && event.Keysym.Sym == sdl.K_q {
					running = false
					break
				}
			case *sdl.MouseWheelEvent:
				e := ScrollEvent{}
				if event.Y != 0 {
					if event.Y > 0 {
						e.Direction = Up
						e.Delta = float64(event.Y)
					} else if event.Y < 0 {
						e.Direction = Down
						e.Delta = -float64(event.Y)
					}

					x, y, _ := sdl.GetMouseState()
					element := hitTest(windowElement, float64(x), float64(y))
					if element != nil {
						bubbleUp(element, e)
					}
				}
			case *sdl.WindowEvent:
				if event.Event == sdl.WINDOWEVENT_RESIZED {
					updateWindowSize(float64(event.Data1), float64(event.Data2))
				}
			}
		}
	}

	return nil
}

func inBounds(offset Offset, size Size, x, y float64) bool {
	xo := x - offset.x
	yo := y - offset.y
	return xo > 0 && yo > 0 && xo < size.Width && yo < size.Height
}

func hitTest(element Element, x, y float64) Element {
	offset := element.getOffset()

	if inBounds(offset, element.getSize(), x, y) {
		for _, child := range getElementChildren(element) {
			// translate x and y
			xo := x - offset.x
			yo := y - offset.y

			r := hitTest(child, xo, yo)
			if r != nil {
				// child was in bounds too
				// child returned deepest hit
				return r
			}
		}
		// this is widget is the deepest hit
		return element
	} else {
		// not in bounds, skip our whole tree.
		return nil
	}
}

func bubbleUp(element Element, event PointerEvent) {
	if pel, ok := element.(PointerEventListener); ok {
		if pel.HandleEvent(event) {
			// event handled
			return
		}
	}

	parent := element.getParentElement()
	if parent != nil {
		bubbleUp(parent, event)
	}
}

/* recurse the element tree, rebuilding any subtree that is marked dirty */
func rebuildDirty(element Element) error {
	statefulElement, ok := element.(*StatefulElement)
	if ok && !statefulElement.built {
		// this follows the children implicitely
		newElement, err := buildElementTree(element.GetWidget(), element)
		if err != nil {
			return err
		}
		if statefulElement != newElement.(*StatefulElement) {
			panic("statefulelement changed element during dirty rebuild")
		}
		statefulElement.rendered = false
	} else {
		// just follow the children.
		var children = getElementChildren(element)
		for _, child := range children {
			err := rebuildDirty(child)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func floatUpRendered(element Element) bool {
	compositeElement, ok := element.(*CompositeElement)

	var children = getElementChildren(element)
	var rendered = true
	for _, child := range children {
		r := floatUpRendered(child)
		if !r {
			if ok {
				compositeElement.rendered = false
			}
			rendered = false
		}
	}
	if rendered == false {
		return rendered
	}

	statefulElement, ok2 := element.(*StatefulElement)
	if ok && !compositeElement.rendered {
		return false
	} else if ok2 && !statefulElement.rendered {
		statefulElement.rendered = true
		return false
	} else {
		return true
	}
}

func elementFromStatelessWidget(sw StatelessWidget, oldElement Element) (Element, error) {
	oldStatelessElement, ok := oldElement.(*StatelessElement)

	// reusing the existing element
	var element *StatelessElement
	if ok && sameType(sw, oldStatelessElement.GetWidget()) {
		element = oldStatelessElement
		element.updateWidget(sw)
	} else {
		element = NewStatelessElement(sw)
	}

	// rebuild the widget
	builtWidget, err := sw.Build(element)
	if err != nil {
		return nil, err
	}

	// check on the children
	var oldChildElement Element
	if oldStatelessElement != nil {
		oldChildElement = oldStatelessElement.child
	}

	childElement, err := buildElementTree(builtWidget, oldChildElement)
	if err != nil {
		return nil, err
	}
	element.setChildElement(element, childElement)

	return element, nil
}

func elementFromStatefulWidget(widget StatefulWidget, oldElement Element) (Element, error) {
	oldStatefulElement, ok := oldElement.(*StatefulElement)

	// reusing the existing element & state?
	var element *StatefulElement
	if ok && sameType(widget, oldStatefulElement.GetWidget()) {
		element = oldStatefulElement
		element.updateWidget(widget)
	} else {
		state := widget.CreateState()
		element = NewStatefulElement(widget, state)
	}

	// build the state
	childWidget, err := element.state.Build(element)
	if err != nil {
		return nil, err
	}

	// build the child subtree
	var oldChildElement Element
	if oldStatefulElement != nil {
		oldChildElement = oldStatefulElement.child
	}
	childElement, err := buildElementTree(childWidget, oldChildElement)
	if err != nil {
		return nil, err
	}
	element.setChildElement(element, childElement)
	element.built = true
	return element, nil
}

func sameType(a, b interface{}) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}

func getWidgetChildren(ew ElementWidget) []Widget {
	if parent, ok := ew.(HasChild); ok {
		child := parent.getChild()
		if child == nil {
			return []Widget{}
		} else {
			return []Widget{parent.getChild()}
		}
	} else if parent, ok := ew.(HasChildren); ok {
		return parent.getChildren()
	}
	return []Widget{}
}

func setElementChildren(e Element, ec []Element) {
	if parent, ok := e.(HasChildElement); ok {
		if len(ec) > 1 {
			panic("unhandleable")
		}
		if len(ec) == 1 {
			parent.setChildElement(e, ec[0])
		}
	} else if parent, ok := e.(HasChildrenElements); ok {
		parent.setChildrenElements(e, ec)
	}
}

func getElementChildren(e Element) []Element {
	if parent, ok := e.(HasChildElement); ok {
		child := parent.getChildElement()
		if child == nil {
			return []Element{}
		} else {
			return []Element{child}
		}
	} else if parent, ok := e.(HasChildrenElements); ok {
		return parent.getChildrenElements()
	}
	return []Element{}
}

func processElementChildren(widget ElementWidget, newElement Element, oldElement Element) error {

	widgetChildren := getWidgetChildren(widget)
	newElementChildren := make([]Element, 0, len(widgetChildren))
	oldElementChildren := getElementChildren(oldElement)

	var i int
	for _, widgetChild := range widgetChildren {

		// check if we have an old element to reuse (for keeping state)
		var oldChildElement Element
		if len(oldElementChildren) > i && sameType(widgetChild, oldElementChildren[i].GetWidget()) {
			oldChildElement = oldElementChildren[i]
		}

		newChildElement, err := buildElementTree(widgetChild, oldChildElement)
		if err != nil {
			return err
		}
		if newChildElement != nil {
			newElementChildren = append(newElementChildren, newChildElement)
		}

		i++
	}
	setElementChildren(newElement, newElementChildren)
	return nil
}

func elementFromElementWidget(ew ElementWidget, oldElement Element) (Element, error) {

	// reusing the existing element & state?
	var element Element
	if oldElement != nil && sameType(ew, oldElement.GetWidget()) {
		element = oldElement
		element.updateWidget(ew)
	} else {
		element = ew.createElement()
	}

	err := processElementChildren(ew, element, oldElement)
	return element, err
}

func buildElementTree(w Widget, oldElement Element) (Element, error) {

	if w == nil {
		return nil, nil
	}

	/* if reflect.ValueOf(w).Kind() != reflect.Ptr {
		return nil, errors.New(fmt.Sprintf("widget in tree is not a pointer, type %T, value %v", w, w))
	} */

	if b, ok := w.(StatelessWidget); ok {
		return elementFromStatelessWidget(b, oldElement)
	} else if sw, ok := w.(StatefulWidget); ok {
		return elementFromStatefulWidget(sw, oldElement)
	} else if ew, ok := w.(ElementWidget); ok {
		return elementFromElementWidget(ew, oldElement)
	} else {
		return nil, errors.New(fmt.Sprintf("unknown widget type in tree, type %T, value %v", w, w))
	}
}

func ContextOf(context BuildContext, typeOf interface{}) BuildContext {
	element := context.(Element)
	for parentContext := element.getParentElement(); parentContext != nil; parentContext = element.getParentElement() {
		widget := parentContext.GetWidget()
		if sameType(widget, typeOf) {
			return parentContext
		}
	}
	return nil
}
