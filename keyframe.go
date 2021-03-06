package goga

import (
	"github.com/go-gl/gl/v3.2-core/gl"
)

const (
	keyframe_sprite_renderer_name = "keyframeRenderer"
)

// A single keyframe within a keyframe set.
type Keyframe struct {
	texCoord *VBO
	Min, Max Vec2
}

// Creates a new single keyframe with texture VBO.
func NewKeyframe(min, max Vec2) *Keyframe {
	keyframe := &Keyframe{Min: min, Max: max}

	texData := make([]float32, 8)
	texData = []float32{float32(min.X),
		float32(max.Y),
		float32(max.X),
		float32(max.Y),
		float32(min.X),
		float32(min.Y),
		float32(max.X),
		float32(min.Y)}

	keyframe.texCoord = NewVBO(gl.ARRAY_BUFFER)
	keyframe.texCoord.Fill(gl.Ptr(texData), 4, 8, gl.STATIC_DRAW)

	CheckGLError()

	return keyframe
}

// A set of keyframes making up an animation.
type KeyframeSet struct {
	Keyframes []Keyframe
}

// Creates a new empty keyframe set with given size.
func NewKeyframeSet() *KeyframeSet {
	set := &KeyframeSet{}
	set.Keyframes = make([]Keyframe, 0)

	return set
}

// Adds a new keyframe to set and returns new length.
func (s *KeyframeSet) Add(frame *Keyframe) int {
	s.Keyframes = append(s.Keyframes, *frame)
	return len(s.Keyframes)
}

// Keyframe animation component.
// It has a start and an end frame, a play speed and option to loop.
type KeyframeAnimation struct {
	Start, End    int
	Loop          bool
	Speed         float64
	Current       int
	Interpolation float64
}

// Creates a new keyframe animation with given start, end and loop.
func NewKeyframeAnimation(start, end int, loop bool, speed float64) *KeyframeAnimation {
	return &KeyframeAnimation{start, end, loop, speed, 0, 0}
}

// An animated sprite is a sprite with keyframe animation information.
// It will be updated and rendered by the KeyframeRenderer.
type AnimatedSprite struct {
	*Actor
	*Pos2D
	*Tex
	*KeyframeSet
	*KeyframeAnimation
}

// Creates a new animated sprite.
func NewAnimatedSprite(tex *Tex, set *KeyframeSet, width, height int) *AnimatedSprite {
	sprite := &AnimatedSprite{}
	sprite.Actor = NewActor()
	sprite.Pos2D = NewPos2D()
	sprite.Tex = tex
	sprite.KeyframeSet = set
	sprite.Scale = Vec2{1, 1}
	sprite.Visible = true

	if width > 0 && height > 0 {
		sprite.Size = Vec2{float64(width), float64(height)}
	} else {
		sprite.Size = Vec2{tex.GetSize().X, tex.GetSize().Y}
	}

	CheckGLError()

	return sprite
}

// The keyframe renderer renders animated sprites.
// It has a 2D position component, to move all sprites at once.
type KeyframeRenderer struct {
	Pos2D

	Shader *Shader
	Camera *Camera

	sprites       []AnimatedSprite
	index, vertex *VBO
	vao           *VAO
}

// Creates a new keyframe renderer using given shader and camera.
// If shader and/or camera are nil, the default one will be used.
func NewKeyframeRenderer(shader *Shader, camera *Camera) *KeyframeRenderer {
	if shader == nil {
		shader = Default2DShader
	}

	if camera == nil {
		camera = DefaultCamera
	}

	var tc *VBO

	renderer := &KeyframeRenderer{}
	renderer.Shader = shader
	renderer.Camera = camera
	renderer.sprites = make([]AnimatedSprite, 0)
	renderer.index, renderer.vertex, tc = CreateRectMesh(false)
	tc.Drop() // we don't need that VBO
	renderer.Size = Vec2{1, 1}
	renderer.Scale = Vec2{1, 1}

	renderer.vao = NewVAO()
	renderer.vao.Bind()
	renderer.Shader.EnableVertexAttribArrays()
	renderer.index.Bind()
	renderer.vertex.Bind()
	renderer.vertex.AttribPointer(shader.GetAttribLocation(Default_shader_2D_vertex_attrib), 2, gl.FLOAT, false, 0)
	renderer.vao.Unbind()

	CheckGLError()

	return renderer
}

// Frees recources created by keyframe renderer.
// This is called automatically when system gets removed.
func (s *KeyframeRenderer) Cleanup() {
	s.index.Drop()
	s.vertex.Drop()
	s.vao.Drop()
}

// Adds animated sprite to the renderer.
func (s *KeyframeRenderer) Add(actor *Actor, pos *Pos2D, tex *Tex, set *KeyframeSet, animation *KeyframeAnimation) bool {
	id := actor.GetId()

	for _, sprite := range s.sprites {
		if id == sprite.Actor.GetId() {
			return false
		}
	}

	s.sprites = append(s.sprites, AnimatedSprite{actor, pos, tex, set, animation})

	return true
}

// Removes animated sprite from renderer.
func (s *KeyframeRenderer) Remove(actor *Actor) bool {
	return s.RemoveById(actor.GetId())
}

// Removes sprite from renderer by ID.
func (s *KeyframeRenderer) RemoveById(id ActorId) bool {
	for i, sprite := range s.sprites {
		if sprite.Actor.GetId() == id {
			s.sprites = append(s.sprites[:i], s.sprites[i+1:]...)
			return true
		}
	}

	return false
}

// Removes all animated sprites.
func (s *KeyframeRenderer) RemoveAll() {
	s.sprites = make([]AnimatedSprite, 0)
}

// Returns number of sprites.
func (s *KeyframeRenderer) Len() int {
	return len(s.sprites)
}

func (s *KeyframeRenderer) GetName() string {
	return keyframe_sprite_renderer_name
}

// Updates animation state and renders sprites.
func (s *KeyframeRenderer) Update(delta float64) {
	// update animation state
	for _, sprite := range s.sprites {
		if sprite.KeyframeAnimation == nil {
			continue
		}

		sprite.Interpolation += delta * sprite.KeyframeAnimation.Speed

		if sprite.Interpolation > 1 {
			sprite.Interpolation = 0
			sprite.Current++

			if sprite.Current > sprite.KeyframeAnimation.End {
				if sprite.KeyframeAnimation.Loop {
					sprite.Current = sprite.KeyframeAnimation.Start
				} else {
					sprite.Current = sprite.KeyframeAnimation.End
				}
			}
		}
	}

	// render
	s.Shader.Bind()
	s.Shader.SendMat3(Default_shader_2D_ortho, *MultMat3(s.Camera.CalcOrtho(), s.CalcModel()))
	s.Shader.SendUniform1i(Default_shader_2D_tex, 0)
	s.vao.Bind()

	for _, sprite := range s.sprites {
		if !sprite.Visible {
			continue
		}

		texCoord := sprite.KeyframeSet.Keyframes[sprite.Current].texCoord
		texCoord.Bind()
		texCoord.AttribPointer(s.Shader.GetAttribLocation(Default_shader_2D_texcoord_attrib), 2, gl.FLOAT, false, 0)

		s.Shader.SendMat3(Default_shader_2D_model, *sprite.CalcModel())
		sprite.Tex.Bind()

		gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, nil)
	}
}
