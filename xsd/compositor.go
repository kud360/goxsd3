package xsd

// Particle is the interface for items that can appear inside a compositor.
// Implemented by Element, GroupRef, Any, Sequence, Choice, and All.
type Particle interface {
	isParticle()
}

// Compositor is the interface for model group containers (sequence, choice, all).
type Compositor interface {
	Particle
	Content
	Particles() []Particle
}

// Sequence represents an xs:sequence compositor.
type Sequence struct {
	MinOccurs int
	MaxOccurs int // -1 = unbounded
	Items     []Particle
}

func (s *Sequence) Particles() []Particle { return s.Items }
func (*Sequence) isParticle()             {}
func (*Sequence) isContent()              {}

// Choice represents an xs:choice compositor.
type Choice struct {
	MinOccurs int
	MaxOccurs int // -1 = unbounded
	Items     []Particle
}

func (c *Choice) Particles() []Particle { return c.Items }
func (*Choice) isParticle()             {}
func (*Choice) isContent()              {}

// All represents an xs:all compositor.
type All struct {
	MinOccurs int
	MaxOccurs int // -1 = unbounded (XSD 1.1 allows > 1)
	Items     []Particle
}

func (a *All) Particles() []Particle { return a.Items }
func (*All) isParticle()             {}
func (*All) isContent()              {}

// Ensure Element implements Particle (defined in model.go).
func (*Element) isParticle() {}
