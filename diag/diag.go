package diag

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

type Cluster struct {
	Key        string
	Name       string
	Parent     *Cluster
	Children   []*Cluster
	Nodes      []*Node
	Components []*Component
}

func (c *Cluster) FullName() string {
	return fmt.Sprintf("%s:%s", c.Key, c.Name)
}

type Clusters []*Cluster

func (cs Clusters) Find(key, name string) *Cluster {
	for _, c := range cs {
		if c.Key == key && c.Name == name {
			return c
		}
	}
	return nil
}

func (cs Clusters) FindByKey(key string) Clusters {
	result := Clusters{}
	for _, c := range cs {
		if c.Key == key {
			result = append(result, c)
		}
	}
	return result
}

var cCache Clusters

type Diag struct {
	Nodes            []*Node    `yaml:"nodes"`
	Networks         []*Network `yaml:"networks"`
	Nests            []string   `yaml:"nests"`
	realNodes        []*RealNode
	clusters         Clusters
	globalComponents []*Component
}

func (d *Diag) Clusters() Clusters {
	return d.clusters
}

func (d *Diag) GlobalComponents() []*Component {
	return d.globalComponents
}

func (d *Diag) BuildNestedClusters(clusterKeys []string) (Clusters, []*Node, error) {
	return buildNestedClusters(d.Clusters(), clusterKeys, d.Nodes)
}

func (d *Diag) classifyComponents() error {

	gc := map[string]struct{}{}
	nc := map[string]struct{}{}
	cc := map[string]struct{}{}
	for _, nw := range d.Networks {
		switch strings.Count(nw.Head, ":") {
		case 2: // cluster components
			cc[nw.Head] = struct{}{}
		case 1: // node components
			nc[nw.Head] = struct{}{}
		case 0: // global components
			gc[nw.Head] = struct{}{}
		}

		switch strings.Count(nw.Tail, ":") {
		case 2: // cluster components
			cc[nw.Tail] = struct{}{}
		case 1: // node components
			nc[nw.Tail] = struct{}{}
		case 0: // global components
			gc[nw.Tail] = struct{}{}
		}
	}

	// global components
	for c := range gc {
		d.globalComponents = append(d.globalComponents, &Component{
			Name: c,
		})
	}

	// node components
	for c := range nc {
		belongTo := false
		for _, n := range d.Nodes {
			for _, com := range n.Components {
				if strings.ToLower(com.FullName()) == strings.ToLower(c) {
					belongTo = true
				}
			}
		}
		if !belongTo {
			splitted := strings.Split(c, ":")
			return fmt.Errorf("node '%s' not found: %s", splitted[0], c)
		}
	}

	// cluster components
	for c := range cc {
		splitted := strings.Split(c, ":")
		clName := fmt.Sprintf("%s:%s", splitted[0], splitted[1])
		comName := splitted[2]
		belongTo := false
		for _, cl := range d.Clusters() {
			if strings.ToLower(cl.FullName()) == strings.ToLower(clName) {
				cl.Components = append(cl.Components, &Component{
					Cluster: cl,
					Name:    comName,
				})
				belongTo = true
				break
			}
		}
		if !belongTo {
			return fmt.Errorf("cluster '%s' not found: %s", clName, c)
		}
	}
	return nil
}

func buildNestedClusters(clusters Clusters, clusterKeys []string, nodes []*Node) (Clusters, []*Node, error) {
	if len(clusterKeys) == 0 {
		return clusters, nodes, nil
	}
	leaf := clusterKeys[len(clusterKeys)-1]
	clusterKeys = clusterKeys[:len(clusterKeys)-1]

	remain := []*Node{}
	belongTo := []*Node{}
	for _, n := range nodes {
		c := n.Clusters.FindByKey(leaf)
		if len(c) == 0 {
			remain = append(remain, n)
			continue
		}
		if len(c) > 1 {
			return nil, nil, fmt.Errorf("duplicate cluster key %s", leaf)
		}
		belongTo = append(belongTo, n)
		if len(clusterKeys) == 0 {
			continue
		}

		// build cluster tree using Node.Clusters
		parent := ""
		var pc Clusters
		for i := 1; i <= len(clusterKeys); i++ {
			parent = clusterKeys[len(clusterKeys)-i]
			pc = n.Clusters.FindByKey(parent)
			if len(pc) > 1 {
				return nil, nil, fmt.Errorf("duplicate cluster key %s", parent)
			}
			if len(pc) == 0 {
				continue
			}
			// _, _ = fmt.Fprintf(os.Stderr, "build cluster tree %v->%v\n", pc[0].FullName(), c[0].FullName())
			c[0].Parent = pc[0]
			pc[0].Children = append(pc[0].Children, c[0])
			c = pc
		}
	}

	// build a direct member node of a cluster
	for _, c := range clusters {
		if c.Key == leaf {
			continue
		}
		nodes := []*Node{}
	N:
		for _, n := range c.Nodes {
			for _, b := range belongTo {
				if n.FullName() == b.FullName() {
					continue N
				}
			}
			nodes = append(nodes, n)
		}
		c.Nodes = nodes
	}

	// build root clusters
	if len(clusterKeys) == 0 {
		root := Clusters{}
		for _, c := range clusters {
			if c.Parent == nil && (c.Key == leaf || len(c.Nodes) > 0) {
				root = append(root, c)
			}
		}
		clusters = root
	}

	return buildNestedClusters(clusters, clusterKeys, remain)
}

type Edge interface {
	FullName() string
}

type Node struct {
	Name       string       `yaml:"name"`
	Desc       string       `yaml:"desc"`
	Components []*Component `yaml:"components,omitempty"`
	Clusters   Clusters     `yaml:"clusters,omitempty"`
	RealNodes  []*RealNode
	nameRe     *regexp.Regexp
}

func (n *Node) FullName() string {
	return n.Name
}

type Network struct {
	Head string
	Tail string
}

type RealNode struct {
	Name string
	Node *Node
}

type Component struct {
	Name    string
	Cluster *Cluster
	Node    *Node
}

func (c *Component) FullName() string {
	if c.Node == nil {
		return fmt.Sprintf("%s:%s", c.Cluster.FullName(), c.Name)
	}
	return fmt.Sprintf("%s:%s", c.Node.FullName(), c.Name)
}

func New() *Diag {
	return &Diag{}
}

func (d *Diag) LoadConfig(in []byte) error {
	if err := yaml.Unmarshal(in, d); err != nil {
		return err
	}
	d.clusters = cCache
	return nil
}

func (d *Diag) LoadConfigFile(path string) error {
	buf, err := loadFile(path)
	if err != nil {
		return err
	}
	return d.LoadConfig(buf)
}

func (d *Diag) LoadRealNodes(in []byte) error {
	if len(d.Nodes) == 0 {
		return errors.New("nodes not found")
	}
	rNodes := []string{}
	if err := yaml.Unmarshal(in, &rNodes); err != nil {
		return err
	}
	for _, rn := range rNodes {
		belongTo := false
		newRn := &RealNode{
			Name: rn,
		}
	N:
		for _, n := range d.Nodes {
			if n.nameRe.MatchString(rn) {
				belongTo = true
				newRn.Node = n
				n.RealNodes = append(n.RealNodes, newRn)
				break N
			}
		}
		if !belongTo {
			return fmt.Errorf("there is a real node '%s' that does not belong to a node", newRn.Name)
		}
		d.realNodes = append(d.realNodes, newRn)
	}
	if err := checkUniqueReadNodes(d.realNodes); err != nil {
		return err
	}
	d.clusters = cCache
	if err := d.classifyComponents(); err != nil {
		return err
	}
	return nil
}

func checkUniqueReadNodes(rNodes []*RealNode) error {
	m := map[string]struct{}{}
	for _, rn := range rNodes {
		if _, exist := m[rn.Name]; exist {
			return fmt.Errorf("duplicate real node name: %s", rn.Name)
		}
		m[rn.Name] = struct{}{}
	}
	return nil
}

func (d *Diag) LoadRealNodesFile(path string) error {
	buf, err := loadFile(path)
	if err != nil {
		return err
	}
	return d.LoadRealNodes(buf)
}

func loadFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadFile(filepath.Clean(fullPath))
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func parseClusterLabel(label string) (*Cluster, error) {
	if !strings.Contains(label, ":") {
		return nil, fmt.Errorf("invalid cluster format: %s", label)
	}
	splitted := strings.Split(label, ":")
	if len(splitted) != 2 {
		return nil, fmt.Errorf("invalid cluster format: %s", label)
	}
	key := splitted[0]
	name := splitted[1]
	current := cCache.Find(key, name)
	if current != nil {
		return current, nil
	}
	newC := &Cluster{
		Key:  key,
		Name: name,
	}
	cCache = append(cCache, newC)
	return newC, nil
}

func unique(in []string) []string {
	m := map[string]struct{}{}
	for _, s := range in {
		m[s] = struct{}{}
	}
	u := []string{}
	for s := range m {
		u = append(u, s)
	}
	return u
}
