package diag

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

func (d *Diag) UnmarshalYAML(data []byte) error {
	raw := struct {
		Nodes    []*Node    `yaml:"nodes"`
		Networks [][]string `yaml:"networks"`
	}{}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Nodes = raw.Nodes
	for _, nw := range raw.Networks {
		if len(nw) != 2 {
			return fmt.Errorf("invalid network format: %s", nw)
		}
		d.Networks = append(d.Networks, &Network{
			Head: nw[0],
			Tail: nw[1],
		})
	}
	return nil
}

func (n *Node) UnmarshalYAML(data []byte) error {
	raw := struct {
		Name       string   `yaml:"name"`
		Desc       string   `yaml:"desc"`
		Components []string `yaml:"components,omitempty"`
		Clusters   []string `yaml:"clusters,omitempty"`
	}{}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	n.Name = raw.Name
	n.nameRe = regexp.MustCompile(fmt.Sprintf("^%s$", strings.Replace(n.Name, "*", ".+", -1)))
	n.Desc = raw.Desc
	n.Components = []*Component{}
	for _, c := range raw.Components {
		n.Components = append(n.Components, &Component{
			Name: c,
			Node: n,
		})
	}
	for _, c := range raw.Clusters {
		cluster, err := parseClusterLabel(c)
		if err != nil {
			return err
		}
		cluster.Nodes = append(cluster.Nodes, n)
		n.Clusters = append(n.Clusters, cluster)
	}

	return nil
}
