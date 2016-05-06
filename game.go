package goga

import (
	"github.com/go-gl/gl/v4.5-core/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
	"log"
	"math"
	"runtime"
	"time"
)

const (
	default_width         = uint32(600)
	default_height        = uint32(400)
	default_title         = "Game"
	default_exit_on_close = true

	// constants for default 2D shader
	Default_shader_2D_vertex_attrib   = "vertex"
	Default_shader_2D_texcoord_attrib = "texCoord"
	Default_shader_2D_ortho           = "o"
	Default_shader_2D_model           = "m"
	Default_shader_2D_tex             = "tex"

	default_shader_2d_vertex_src = `#version 130
		uniform mat3 o, m;
		in vec2 vertex;
		in vec2 texCoord;
		out vec2 tc;
		void main(){
		tc = texCoord;
		gl_Position = vec4(o*m*vec3(vertex, 1.0), 1.0);
		}`
	default_shader_2d_fragment_src = `#version 130
		precision highp float;
		uniform sampler2D tex;
		in vec2 tc;
		out vec4 color;
		void main(){
		color = texture(tex, tc);
		}`
)

// If set in RunOptions, the function will be called on window resize.
type ResizeCallback func(width, height int)

// Run options allow to set some parameters on startup.
type RunOptions struct {
	Title               string
	Width               uint32
	Height              uint32
	ClearColor          Vec4
	Resizable           bool
	SetViewportOnResize bool
	ResizeCallbackFunc  ResizeCallback
	ExitOnClose         bool
}

// Main game object.
// Setup will be called before the main loop and after GL context has been created.
// Update will be called each frame. This can be used to switch scenes or end game on win state.
// For game logic, System should be used.
type Game interface {
	Setup()
	Update(float64)
}

var (
	running        = true
	clearColor     = Vec4{}
	clearBuffer    []uint32
	viewportWidth  int
	viewportHeight int

	// Default resources
	DefaultCamera   *Camera
	Default2DShader *Shader
)

func init() {
	// GL functions must be called from main thread.
	log.Print("Locking OS thread")
	runtime.LockOSThread()
}

// Creates a new window with given options and starts the game.
// The game struct must implement the Game interface.
// If options is nil, the default options will be used.
// This function will panic on error.
func Run(game Game, options *RunOptions) {
	// init GL
	log.Print("Initializing GL")

	if err := gl.Init(); err != nil {
		panic("Error initializing GL: " + err.Error())
	}

	// init glfw
	log.Print("Initializing GLFW")

	if err := glfw.Init(); err != nil {
		panic("Error initializing GLFW: " + err.Error())
	}

	defer glfw.Terminate()

	// create window
	log.Print("Creating window")
	width := default_width
	height := default_height
	title := default_title
	exitOnClose := default_exit_on_close

	if options != nil && options.Width > 0 {
		width = options.Width
	}

	if options != nil && options.Height > 0 {
		height = options.Height
	}

	if options != nil && len(options.Title) > 0 {
		title = options.Title
	}

	if options != nil {
		exitOnClose = options.ExitOnClose
	}

	if options != nil && !options.Resizable {
		glfw.WindowHint(glfw.Resizable, glfw.False)
	}

	wnd, err := glfw.CreateWindow(int(width), int(height), title, nil, nil)

	if err != nil {
		panic("Error creating GLFW window: " + err.Error())
	}

	// window event handlers
	wnd.SetSizeCallback(func(w *glfw.Window, width, height int) {
		if options == nil {
			SetViewport(0, 0, int32(width), int32(height))
		} else if options != nil && options.SetViewportOnResize {
			SetViewport(0, 0, int32(width), int32(height))
		}

		if activeScene != nil {
			activeScene.Resize(width, height)
		}

		if options != nil && options.ResizeCallbackFunc != nil {
			options.ResizeCallbackFunc(width, height)
		}
	})

	// make GL context current
	wnd.MakeContextCurrent()

	// init go-game
	log.Print("Initializing goga")
	initGoga(int(width), int(height))

	if options != nil && options.Width > 0 && options.Height > 0 {
		SetViewport(0, 0, int32(options.Width), int32(options.Height))
	} else {
		SetViewport(0, 0, int32(default_width), int32(default_height))
	}

	if options != nil {
		clearColor = options.ClearColor
	}

	game.Setup()

	// start and loop
	log.Print("Starting main loop")
	delta := time.Duration(0)
	var deltaSec float64

	for running {
		if exitOnClose && wnd.ShouldClose() {
			cleanup()
			return
		}

		start := time.Now()
		gl.ClearColor(float32(clearColor.X), float32(clearColor.Y), float32(clearColor.Z), float32(clearColor.W))

		for _, buffer := range clearBuffer {
			gl.Clear(buffer)
		}

		if !math.IsInf(deltaSec, 0) && !math.IsInf(deltaSec, -1) {
			updateSystems(deltaSec)
			game.Update(deltaSec)
		}

		CheckGLError()

		wnd.SwapBuffers()
		delta = time.Since(start)
		deltaSec = delta.Seconds()
		glfw.PollEvents()
	}
}

func initGoga(width, height int) {
	// default camera
	DefaultCamera = NewCamera(0, 0, width, height)
	DefaultCamera.CalcRatio()
	DefaultCamera.CalcOrtho()

	// default shader
	shader, err := NewShader(default_shader_2d_vertex_src, default_shader_2d_fragment_src)

	if err != nil {
		panic(err)
	}

	Default2DShader = shader
	Default2DShader.BindAttrib(Default_shader_2D_vertex_attrib)
	Default2DShader.BindAttrib(Default_shader_2D_texcoord_attrib)

	// settings and registration
	ClearColorBuffer(true)
	EnableAlphaBlending(true)
	AddLoader(&PngLoader{gl.LINEAR, false})
	AddSystem(NewSpriteRenderer(nil, nil, false))
}

func cleanup() {
	// cleanup resources
	log.Printf("Cleaning up %v resources", len(resources))

	for _, res := range resources {
		if drop, ok := res.(Dropable); ok {
			drop.Drop()
		}
	}

	// cleanup systems
	log.Printf("Cleaning up %v systems", len(systems))

	for _, system := range systems {
		system.Cleanup()
	}

	// cleanup scenes
	log.Printf("Cleaning up %v scenes", len(scenes))

	for _, scene := range scenes {
		scene.Cleanup()
	}

	// cleanup default
	log.Print("Cleaning up default resources")

	Default2DShader.Drop()
}

// Stops the game and closes the window.
func Stop() {
	log.Print("Stopping main loop")
	running = false
}

// Adds color buffer to list of buffers to be cleared.
// If parameter is false, it will be removed.
func ClearColorBuffer(do bool) {
	removeClearBuffer(gl.COLOR_BUFFER_BIT)

	if do {
		clearBuffer = append(clearBuffer, gl.COLOR_BUFFER_BIT)
	}
}

// Adds depth buffer to list of buffers to be cleared.
// If parameter is false, it will be removed.
func ClearDepthBuffer(do bool) {
	removeClearBuffer(gl.DEPTH_BUFFER_BIT)

	if do {
		clearBuffer = append(clearBuffer, gl.DEPTH_BUFFER_BIT)
	}
}

// Enables/Disables alpha blending by source alpha channel.
// BLEND = SRC_ALPHA | ONE_MINUS_SRC_ALPHA
func EnableAlphaBlending(enable bool) {
	if enable {
		gl.Enable(gl.BLEND)
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	} else {
		gl.Disable(gl.BLEND)
	}
}

// Sets GL viewport.
func SetViewport(x, y, width, height int32) {
	viewportWidth = int(width)
	viewportHeight = int(height)
	DefaultCamera.SetViewport(int(x), int(y), viewportWidth, viewportHeight)
	DefaultCamera.CalcRatio()
	DefaultCamera.CalcOrtho()
	gl.Viewport(x, y, width, height)
}

// Sets GL clear color.
func SetClearColor(r, g, b, a float64) {
	clearColor = Vec4{r, g, b, a}
}

// Returns width of viewport.
func GetWidth() int {
	return viewportWidth
}

// Returns height of viewport.
func GetHeight() int {
	return viewportHeight
}

func removeClearBuffer(buffer uint32) {
	for i, buffer := range clearBuffer {
		if buffer == buffer {
			clearBuffer = append(clearBuffer[:i], clearBuffer[i+1:]...)
			return
		}
	}
}
