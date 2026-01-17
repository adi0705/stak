package models

// Branch represents a branch in the stack
type Branch struct {
	Name     string
	Parent   string
	PRNumber int
	Children []*Branch
}

// NewBranch creates a new Branch instance
func NewBranch(name, parent string, prNumber int) *Branch {
	return &Branch{
		Name:     name,
		Parent:   parent,
		PRNumber: prNumber,
		Children: make([]*Branch, 0),
	}
}

// AddChild adds a child branch to this branch
func (b *Branch) AddChild(child *Branch) {
	b.Children = append(b.Children, child)
}

// IsRoot returns true if this branch has no parent (is a root branch)
func (b *Branch) IsRoot() bool {
	return b.Parent == ""
}

// HasChildren returns true if this branch has child branches
func (b *Branch) HasChildren() bool {
	return len(b.Children) > 0
}

// Stack represents the entire branch stack structure
type Stack struct {
	Branches map[string]*Branch // Map of branch name to Branch
	Roots    []*Branch          // Root branches (those with no parent in the stack)
}

// NewStack creates a new Stack instance
func NewStack() *Stack {
	return &Stack{
		Branches: make(map[string]*Branch),
		Roots:    make([]*Branch, 0),
	}
}

// AddBranch adds a branch to the stack
func (s *Stack) AddBranch(branch *Branch) {
	s.Branches[branch.Name] = branch
}

// GetBranch retrieves a branch by name
func (s *Stack) GetBranch(name string) *Branch {
	return s.Branches[name]
}

// BuildRelationships builds parent-child relationships between branches
func (s *Stack) BuildRelationships() {
	// First, identify roots
	s.Roots = make([]*Branch, 0)

	for _, branch := range s.Branches {
		if branch.Parent == "" {
			s.Roots = append(s.Roots, branch)
			continue
		}

		// Find parent and establish relationship
		parent := s.Branches[branch.Parent]
		if parent != nil {
			parent.AddChild(branch)
		} else {
			// Parent not in stack, treat as root
			s.Roots = append(s.Roots, branch)
		}
	}
}
