package skill

import (
	"context"
	"encoding/json"
	"testing"

	skillpkg "github.com/valpere/kvach/internal/skill"
	"github.com/valpere/kvach/internal/tool"
)

type stubLoader struct{}

func (stubLoader) Discover(_ string, _ []string) ([]skillpkg.CatalogEntry, error) {
	return []skillpkg.CatalogEntry{{Name: "demo", Description: "demo skill", Location: "/tmp/demo/SKILL.md"}}, nil
}

func (stubLoader) Activate(name string) (*skillpkg.Skill, error) {
	return &skillpkg.Skill{
		Frontmatter: skillpkg.Frontmatter{Name: name, Description: "demo skill"},
		BaseDir:     "/tmp/demo",
		Body:        "Do demo.",
	}, nil
}

func (stubLoader) ParseFile(_ string) (*skillpkg.Skill, error) { return nil, nil }

func TestActivateSkillUsesLoader(t *testing.T) {
	st := &skillTool{}
	raw, _ := json.Marshal(map[string]any{"name": "demo"})

	res, err := st.Call(context.Background(), raw, &tool.Context{WorkDir: "/tmp", SkillLoader: stubLoader{}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res == nil || res.Content == "" {
		t.Fatal("expected activation XML")
	}
}
