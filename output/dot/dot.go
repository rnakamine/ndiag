package dot

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/elliotchance/orderedmap"
	"github.com/gobuffalo/packr/v2"
	"github.com/k1LoW/ndiag/config"
	"github.com/k1LoW/ndiag/output"
)

type Dot struct {
	config *config.Config
	box    *packr.Box
}

func New(cfg *config.Config) *Dot {
	return &Dot{
		config: cfg,
		box:    packr.New("dot", "./templates"),
	}
}

func (d *Dot) OutputDiagram(wr io.Writer, diag *config.Diagram) error {
	ts, err := d.box.FindString("diagram.dot.tmpl")
	if err != nil {
		return err
	}
	tmpl := template.Must(template.New("diagram").Funcs(output.Funcs(d.config)).Parse(ts))

	clusters, remain, nEdges, err := d.config.BuildNestedClusters(diag.Layers)
	if err != nil {
		return err
	}
	globalComponents := d.config.GlobalComponents()
	clusters, remain, globalComponents, nEdges, err = d.config.PruneClustersByLabels(clusters, remain, globalComponents, nEdges, diag.Labels)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(wr, map[string]interface{}{
		"GraphAttrs":       d.config.Graph.Attrs(),
		"Clusters":         clusters,
		"RemainNodes":      remain,
		"GlobalComponents": globalComponents,
		"Edges":            config.MergeEdges(nEdges),
		"HideUnlinked":     false,
		"HideRealNodes":    d.config.HideRealNodes,
	}); err != nil {
		return err
	}
	return nil
}

func (d *Dot) OutputLayer(wr io.Writer, l *config.Layer) error {
	ts, err := d.box.FindString("diagram.dot.tmpl")
	if err != nil {
		return err
	}
	tmpl := template.Must(template.New("diagram").Funcs(output.Funcs(d.config)).Parse(ts))

	clusters, remain, nEdges, err := d.config.BuildNestedClusters([]string{l.Name})
	if err != nil {
		return err
	}
	edges := []*config.NEdge{}
L:
	for _, e := range nEdges {
		for _, n := range remain {
			// remove rel with global nodes
			if strings.HasPrefix(e.Src.Id(), fmt.Sprintf("%s:", n.Id())) {
				continue L
			}
			if strings.HasPrefix(e.Dst.Id(), fmt.Sprintf("%s:", n.Id())) {
				continue L
			}
		}
		// remove rel with global components
		if (e.Src.Node == nil && e.Src.Cluster == nil) || (e.Dst.Node == nil && e.Dst.Cluster == nil) {
			continue L
		}
		edges = append(edges, e)
	}

	if err := tmpl.Execute(wr, map[string]interface{}{
		"GraphAttrs":       d.config.Graph.Attrs(),
		"Clusters":         clusters,
		"RemainNodes":      []*config.Node{},
		"GlobalComponents": []*config.Component{},
		"Edges":            config.MergeEdges(edges),
		"HideUnlinked":     false,
		"HideRealNodes":    d.config.HideRealNodes,
	}); err != nil {
		return err
	}
	return nil
}

func (d *Dot) OutputNode(wr io.Writer, n *config.Node) error {
	ts, err := d.box.FindString("node.dot.tmpl")
	if err != nil {
		return err
	}
	tmpl := template.Must(template.New("diagram").Funcs(output.Funcs(d.config)).Parse(ts))

	clusters := config.Clusters{}
	cIds := orderedmap.NewOrderedMap() // map[string]*config.Cluster{}
	nodes := []*config.Node{n}
	nIds := orderedmap.NewOrderedMap() // map[string]*config.Node{ n.Id(): n }
	nIds.Set(n.Id(), n)
	globalComponents := []*config.Component{}
	gIds := orderedmap.NewOrderedMap() // map[string]*config.Component{}
	edges := []*config.NEdge{}

	for _, e := range d.config.NEdges() {
		if (e.Src.Node == nil || e.Src.Node.Id() != n.Id()) && (e.Dst.Node == nil || e.Dst.Node.Id() != n.Id()) {
			continue
		}
		switch {
		case e.Src.Node != nil:
			nIds.Set(e.Src.Node.Id(), e.Src.Node)
		case e.Src.Cluster != nil:
			e.Src.Cluster.Nodes = nil
			cIds.Set(e.Src.Cluster.Id(), e.Src.Cluster)
		default:
			gIds.Set(e.Src.Id(), e.Src)
		}
		switch {
		case e.Dst.Node != nil:
			nIds.Set(e.Dst.Node.Id(), e.Dst.Node)
		case e.Dst.Cluster != nil:
			e.Dst.Cluster.Nodes = nil
			cIds.Set(e.Dst.Cluster.Id(), e.Dst.Cluster)
		default:
			gIds.Set(e.Dst.Id(), e.Dst)
		}
		edges = append(edges, e)
	}
	for _, k := range nIds.Keys() {
		n, _ := nIds.Get(k)
		nodes = append(nodes, n.(*config.Node))
	}
	for _, k := range cIds.Keys() {
		c, _ := cIds.Get(k)
		clusters = append(clusters, c.(*config.Cluster))
	}
	for _, k := range gIds.Keys() {
		c, _ := gIds.Get(k)
		globalComponents = append(globalComponents, c.(*config.Component))
	}

	if err := tmpl.Execute(wr, map[string]interface{}{
		"GraphAttrs":       d.config.Graph.Attrs(),
		"MainNodeId":       n.Id(),
		"Clusters":         clusters,
		"RemainNodes":      nodes,
		"GlobalComponents": globalComponents,
		"Edges":            config.MergeEdges(edges),
		"HideRealNodes":    d.config.HideRealNodes,
	}); err != nil {
		return err
	}
	return nil
}

func (d *Dot) OutputLabel(wr io.Writer, l *config.Label) error {
	ts, err := d.box.FindString("diagram.dot.tmpl")
	if err != nil {
		return err
	}
	tmpl := template.Must(template.New("diagram").Funcs(output.Funcs(d.config)).Parse(ts))

	clusters, globalNodes, nEdges, err := d.config.BuildNestedClusters([]string{})
	if err != nil {
		return err
	}

	globalComponents := d.config.GlobalComponents()
	clusters, globalNodes, globalComponents, nEdges, err = d.config.PruneClustersByLabels(clusters, globalNodes, globalComponents, nEdges, []string{l.Name})
	if err != nil {
		return err
	}

	if err := tmpl.Execute(wr, map[string]interface{}{
		"GraphAttrs":       d.config.Graph.Attrs(),
		"Clusters":         clusters,
		"RemainNodes":      globalNodes,
		"GlobalComponents": globalComponents,
		"Edges":            nEdges,
		"HideUnlinked":     false,
		"HideRealNodes":    d.config.HideRealNodes,
	}); err != nil {
		return err
	}
	return nil
}
