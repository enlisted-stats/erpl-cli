package main

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enlisted-stats/erpl-cli/internal/blk"
	"github.com/enlisted-stats/erpl-cli/internal/maps"
	"github.com/enlisted-stats/erpl-cli/internal/vromfs"
)

// gameArchive holds the scenes and templates, relative to the game folder.
var gameArchive = filepath.Join("content", "enlisted", "enlisted-game.vromfs.bin")

const scenesDir = "gamedata/scenes/"

// game reads missions from the game files.
type game struct {
	files     map[string][]byte
	blk       *blk.Decoder
	scenes    map[string]*blk.Block // decoded scenes by path
	minimaps  map[string]minimap    // by template name
	templates map[string]template   // by name
}

// template is what erpl-mapgen needs of an entity template: the templates it is built on
// and the translation key of the name a structure has in the engineer's building menu.
type template struct {
	uses     []string
	building string
}

type minimap struct {
	texture string
	maps.Map
}

func openGame(dir string) (*game, error) {
	files, err := vromfs.Open(filepath.Join(dir, gameArchive))
	if err != nil {
		return nil, err
	}
	dec, err := blk.NewDecoder(files)
	if err != nil {
		return nil, err
	}
	g := &game{files: files, blk: dec, scenes: map[string]*blk.Block{}, minimaps: map[string]minimap{}, templates: map[string]template{}}
	g.readTemplates()
	return g, nil
}

// readTemplates collects the entity templates. They are spread over the gamedata
// files; files that are not binary BLK are skipped.
func (g *game) readTemplates() {
	for name, data := range g.files {
		if !strings.HasPrefix(name, "gamedata/") || strings.HasPrefix(name, scenesDir) || !strings.HasSuffix(name, ".blk") {
			continue
		}
		root, err := g.blk.Decode(data)
		if err != nil {
			continue
		}
		for _, t := range root.Blocks {
			tp := template{building: t.String("building_menu__text")}
			for _, p := range t.Params {
				if u, ok := p.Value.(string); ok && p.Name == "_use" {
					tp.uses = append(tp.uses, u)
				}
			}
			if old, ok := g.templates[t.Name]; !ok || old.building == "" {
				g.templates[t.Name] = tp
			}

			tex := strings.TrimSuffix(t.String("mapTex"), "*")
			if tex == "" {
				continue
			}
			m := minimap{texture: tex}
			m.LeftTop, _ = point2(t, "left_top")
			m.RightBottom, _ = point2(t, "right_bottom")
			if v, ok := t.Get("northAngle"); ok {
				m.NorthAngle, _ = v.(float32)
			}
			g.minimaps[t.Name] = m
		}
	}
}

// buildingKey returns the translation key of the name of a structure template, looking
// through the templates it is built on, the later ones first as they override; it is
// empty if the template is not a building.
func (g *game) buildingKey(name string) string {
	seen := map[string]bool{}
	var find func(string) string
	find = func(n string) string {
		t, ok := g.templates[n]
		if !ok || seen[n] {
			return ""
		}
		seen[n] = true
		if t.building != "" {
			return t.building
		}
		for i := len(t.uses) - 1; i >= 0; i-- {
			if k := find(t.uses[i]); k != "" {
				return k
			}
		}
		return ""
	}
	return find(name)
}

// missionNames lists the scenes of the scenes folder that are missions, not parts imported by them.
func (g *game) missionNames() []string {
	var names []string
	for name := range g.files {
		m, ok := strings.CutPrefix(name, scenesDir)
		if ok && strings.HasSuffix(m, ".blk") && !strings.Contains(m, "/") && !strings.HasPrefix(m, "_") {
			names = append(names, strings.TrimSuffix(m, ".blk"))
		}
	}
	slices.Sort(names)
	return names
}

// entities returns the entities of a scene and of the scenes it imports, the scene's own first.
func (g *game) entities(scene string) ([]*blk.Block, error) {
	var es []*blk.Block
	seen := map[string]bool{}
	for todo := []string{scene}; len(todo) > 0; todo = todo[1:] {
		p := todo[0]
		if seen[p] {
			continue
		}
		seen[p] = true
		root, err := g.scene(p)
		if err != nil {
			return nil, err
		}
		for _, b := range root.Blocks {
			switch b.Name {
			case "import":
				if s := b.String("scene"); s != "" {
					todo = append(todo, path.Clean(s))
				}
			case "entity":
				es = append(es, b)
			}
		}
	}
	return es, nil
}

func (g *game) scene(p string) (*blk.Block, error) {
	if s, ok := g.scenes[p]; ok {
		return s, nil
	}
	data, ok := g.files[p]
	if !ok {
		return nil, fmt.Errorf("scene %s not found", p)
	}
	s, err := g.blk.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("scene %s: %w", p, err)
	}
	g.scenes[p] = s
	return s, nil
}

// mission returns the minimap template and the battle areas of a mission; the
// template is empty if the mission has no minimap.
func (g *game) mission(name string) (string, []maps.Area, error) {
	es, err := g.entities(scenesDir + name + ".blk")
	if err != nil {
		return "", nil, err
	}
	mm := ""
	for _, e := range es {
		for t := range strings.SplitSeq(e.String("_template"), "+") {
			if _, ok := g.minimaps[t]; ok && mm == "" {
				mm = t
			}
		}
	}
	return mm, battleAreas(es), nil
}

// link is a polygon point entity: its position and the id of the next point.
type link struct {
	pos  [2]float32
	next string
}

// battleAreas reads the battle areas among the entities. A polygon has its points
// either in a list or as separate point entities chained by ids.
func battleAreas(es []*blk.Block) []maps.Area {
	chains := map[string]map[string]link{} // area id → point id → link
	for _, e := range es {
		if templateOf(e) != "battle_area_polygon_point" {
			continue
		}
		area := e.String("battle_area_polygon_point__forBattleAreaId")
		if chains[area] == nil {
			chains[area] = map[string]link{}
		}
		chains[area][e.String("battle_area_polygon_point__id")] = link{groundPos(e), e.String("battle_area_polygon_point__nextId")}
	}

	var areas []maps.Area
	for _, e := range es {
		a := maps.Area{Team: -1, Active: true, Group: e.String("groupName")}
		if v, ok := e.Get("battle_area__team"); ok {
			if t, ok := v.(int32); ok {
				a.Team = int(t)
			}
		}
		if v, ok := e.Get("active"); ok {
			a.Active, _ = v.(bool)
		}
		switch templateOf(e) {
		case "poly_battle_area":
			if list := listBlock(e, "poly_area__points"); list != nil {
				for _, p := range list.Params {
					if v, ok := p.Value.([3]float32); ok {
						a.Points = append(a.Points, [2]float32{v[0], v[2]})
					}
				}
			} else {
				a.Points = chain(chains[e.String("battleAreaId")])
			}
		case "box_battle_area":
			a.Points = box(e)
		case "aircraft_box_battle_area":
			a.Air, a.Points = true, box(e)
		default:
			continue
		}
		if len(a.Points) >= 3 {
			areas = append(areas, a)
		}
	}
	return areas
}

// chain follows the points from the smallest id until the chain loops or breaks.
func chain(links map[string]link) [][2]float32 {
	if len(links) == 0 {
		return nil
	}
	id := ""
	for k := range links {
		if id == "" || k < id {
			id = k
		}
	}
	var pts [][2]float32
	for range len(links) {
		l, ok := links[id]
		if !ok {
			break
		}
		pts = append(pts, l.pos)
		id = l.next
	}
	return pts
}

// box returns the corners of a unit box scaled and placed by the entity transform.
func box(e *blk.Block) [][2]float32 {
	v, _ := e.Get("transform")
	m, ok := v.(blk.TMatrix)
	if !ok {
		return nil
	}
	var pts [][2]float32
	for _, s := range [][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		x := m[3][0] + (s[0]*m[0][0]+s[1]*m[2][0])/2
		z := m[3][2] + (s[0]*m[0][2]+s[1]*m[2][2])/2
		pts = append(pts, [2]float32{x, z})
	}
	return pts
}

// templateOf is the first part of the entity's template.
func templateOf(e *blk.Block) string {
	t, _, _ := strings.Cut(e.String("_template"), "+")
	return t
}

// listBlock returns the nested block of a list parameter; its name carries the list type.
func listBlock(e *blk.Block, name string) *blk.Block {
	for _, b := range e.Blocks {
		if b.Name == name || strings.HasPrefix(b.Name, name+":") {
			return b
		}
	}
	return nil
}

func groundPos(e *blk.Block) [2]float32 {
	v, _ := e.Get("transform")
	m, _ := v.(blk.TMatrix)
	return [2]float32{m[3][0], m[3][2]}
}

func point2(b *blk.Block, name string) ([2]float32, bool) {
	v, _ := b.Get(name)
	p, ok := v.([2]float32)
	return p, ok
}
