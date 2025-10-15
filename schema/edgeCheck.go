package schema

import (
	"fmt"

	"github.com/bmeg/grip/gripql"
)

func (s *Schema) FindOrphanEdges(elements []*gripql.GraphElement) []string {
	//Pulls down indexd records for project, generates docrefs, does edge/vertex generation
	// and then checks to see if there exists any edges which point to vertices that do not exist
	vertexIDs := make(map[string]any)
	for _, el := range elements {
		if el.Vertex != nil && el.Vertex.Id != "" {
			vertexIDs[el.Vertex.Id] = struct{}{}
		}
	}
	var orphans []string
	for _, el := range elements {
		if el.Edge != nil && el.Edge.From != "" && el.Edge.To != "" {
			_, fromExists := vertexIDs[el.Edge.From]
			_, toExists := vertexIDs[el.Edge.To]
			if !fromExists || !toExists {
				orphans = append(orphans, fmt.Sprintf("Orphan edge (ID: %s, From: %s, To: %s) - missing %s", el.Edge.Id, el.Edge.From, el.Edge.To, map[bool]string{true: "to vertex", false: "from vertex"}[!fromExists]))
			}
		}
	}
	return orphans
}
