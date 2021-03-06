package gui

/*
#import <stdint.h>
#import <stdbool.h>

typedef unsigned short unichar;

void startApp();
void stopApp();
uintptr_t newWindow(int, int);
void setGridSize(uintptr_t, int, int);
void drawText(uintptr_t, const char *, int, int, uint8_t, int32_t, int32_t, int32_t);
void drawRepeatedText(uintptr_t, unichar, int, int, uint8_t, int32_t, int32_t, int32_t);
void clearScreen(uintptr_t, int32_t);
void scrollScreen(uintptr_t, int, int, int, int, int, int32_t);
void flush(uintptr_t, int, int, int, const char *, int, uint8_t, int32_t, int32_t, int32_t);
void setTitle(uintptr_t, const char *);
void setIcon(uintptr_t, const char *);
void bell(uintptr_t, bool);
void getCellSize(int*, int*);
*/
import "C"
import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/tweekmonster/nmux/screen"
)

func init() {
	runtime.LockOSThread()
}

var appCallback func()
var windowInputs map[uintptr]chan interface{}

type window struct {
	id     uintptr
	mu     sync.Mutex
	events chan interface{}
}

func (w *window) GetID() uintptr {
	return w.id
}

func (w *window) SetGrid(cols, rows int) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	C.setGridSize(C.uintptr_t(w.id), C.int(cols), C.int(rows))
	return nil
}

func (w *window) PutString(s string, index int, attrs screen.CellAttrs) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	sbytes := []byte(s + "\x00")
	fg := C.int32_t(attrs.Fg)
	bg := C.int32_t(attrs.Bg)
	if attrs.Attrs&screen.AttrReverse != 0 {
		fg, bg = bg, fg
	}

	C.drawText(C.uintptr_t(w.id), (*C.char)(unsafe.Pointer(&sbytes[0])), C.int(len(sbytes)), C.int(index), C.uint8_t(attrs.Attrs), fg, bg, C.int32_t(attrs.Sp))

	return nil
}

func (w *window) PutRepeatedString(r rune, length, index int, attrs screen.CellAttrs) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	fg := C.int32_t(attrs.Fg)
	bg := C.int32_t(attrs.Bg)
	if attrs.Attrs&screen.AttrReverse != 0 {
		fg, bg = bg, fg
	}

	C.drawRepeatedText(C.uintptr_t(w.id), C.unichar(r), C.int(length), C.int(index), C.uint8_t(attrs.Attrs), fg, bg, C.int32_t(attrs.Sp))
	return nil
}

func (w *window) Scroll(delta, top, bottom, left, right int, bg screen.Color) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	C.scrollScreen(C.uintptr_t(w.id), C.int(delta), C.int(top), C.int(bottom), C.int(left), C.int(right), C.int32_t(bg))
	return nil
}

func (w *window) Clear(bg screen.Color) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	C.clearScreen(C.uintptr_t(w.id), C.int32_t(bg))
	return nil
}

func (w *window) Flush(mode int, character string, width int, cursor screen.Vector2, attrs screen.CellAttrs) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	fg := C.int32_t(attrs.Fg)
	bg := C.int32_t(attrs.Bg)
	if attrs.Attrs&screen.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	cbytes := []byte(character + "\x00")
	C.flush(C.uintptr_t(w.id), C.int(mode), C.int(cursor.X), C.int(cursor.Y), (*C.char)(unsafe.Pointer(&cbytes[0])), C.int(width), C.uint8_t(attrs.Attrs), fg, bg, C.int32_t(attrs.Sp))
	return nil
}

func (w *window) SetTitle(title string) error {
	tbytes := append([]byte(title), 0)
	C.setTitle(C.uintptr_t(w.id), (*C.char)(unsafe.Pointer(&tbytes[0])))
	return nil
}

func (w *window) SetIcon(icon string) error {
	ibytes := append([]byte(icon), 0)
	C.setIcon(C.uintptr_t(w.id), (*C.char)(unsafe.Pointer(&ibytes[0])))
	return nil
}

func (w *window) Bell(visual bool) error {
	C.bell(C.uintptr_t(w.id), C.bool(visual))
	return nil
}

func (w *window) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return nil
}

func (w *window) SendEvent(event interface{}) {
	w.events <- event
}

func (w *window) NextEvent() interface{} {
	return <-w.events
}

func (w *window) EventChannel() <-chan interface{} {
	return w.events
}

func platformStart(callback func()) error {
	appCallback = callback
	C.startApp()
	return nil
}

func platformStop() {
	C.stopApp()
}

func platformNewWindow(width, height int) Window {
	win := &window{
		id:     uintptr(C.newWindow(C.int(width), C.int(height))),
		events: make(chan interface{}),
	}
	return win
}

func platformCellSize() screen.Vector2 {
	var x, y int
	C.getCellSize((*C.int)(unsafe.Pointer(&x)), (*C.int)(unsafe.Pointer(&y)))
	return screen.Vector2{X: x, Y: y}
}

//export appStarted
func appStarted() {
	go appCallback()
}

//export appStopped
func appStopped() {
	sendApplicationEvent(StateEvent("stopped"))
}

//export appHidden
func appHidden() {
	sendApplicationEvent(StateEvent("hidden"))
}

//export inputEvent
func inputEvent(id uintptr, key *C.char) {
	sendWindowEvent(id, InputEvent(C.GoString(key)))
}

//export winMoved
func winMoved(id uintptr, x, y C.int) {
	sendWindowEvent(id, MoveEvent{
		X: int(x),
		Y: int(y),
	})
}

//export winResized
func winResized(id uintptr, w, h, gw, gh C.int) {
	sendWindowEvent(id, ResizeEvent{
		Width:      int(w),
		Height:     int(h),
		GridWidth:  int(gw),
		GridHeight: int(gh),
	})
}

//export winClosed
func winClosed(id uintptr) {
	sendWindowEvent(id, StateEvent("closed"))
}

//export winFocused
func winFocused(id uintptr) {
	sendWindowEvent(id, StateEvent("focused"))
}

//export winFocusLost
func winFocusLost(id uintptr) {
	sendWindowEvent(id, StateEvent("lostFocus"))
}

//export appMenuSelected
func appMenuSelected(title *C.char) {
	sendApplicationEvent(MenuEvent(C.GoString(title)))
}

//export windowMenuSelected
func windowMenuSelected(id uintptr, title *C.char) {
	sendWindowEvent(id, MenuEvent(C.GoString(title)))
}
