package architecture

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	svgNodeWidth  = 220
	svgNodeHeight = 52
	svgGapX       = 70
	svgGapY       = 75
	svgMargin     = 45
	svgColumns    = 4
)

type svgPoint struct{ x, y float64 }

// RenderSVG renders a projection as deterministic, self-contained SVG.
func RenderSVG(projection Projection) ([]byte, error) {
	nodes := append([]ProjectionNode(nil), projection.Nodes...)
	edges := append([]ProjectionEdge(nil), projection.Edges...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return projectionStatusRank(edges[i].Status) < projectionStatusRank(edges[j].Status)
	})
	positions := layoutProjectionNodes(nodes)
	if err := validateProjectionEdges(edges, positions); err != nil {
		return nil, err
	}
	return renderProjectionSVG(projection, nodes, edges, positions), nil
}

func layoutProjectionNodes(nodes []ProjectionNode) map[string]svgPoint {
	positions := make(map[string]svgPoint, len(nodes))
	for index, node := range nodes {
		column, row := index%svgColumns, index/svgColumns
		positions[node.ID] = svgPoint{
			x: svgMargin + float64(column*(svgNodeWidth+svgGapX)),
			y: svgMargin + 45 + float64(row*(svgNodeHeight+svgGapY)),
		}
	}
	return positions
}

func validateProjectionEdges(edges []ProjectionEdge, positions map[string]svgPoint) error {
	for _, edge := range edges {
		if _, ok := positions[edge.Source]; !ok {
			return fmt.Errorf("projection edge references missing source node %q", edge.Source)
		}
		if _, ok := positions[edge.Target]; !ok {
			return fmt.Errorf("projection edge references missing target node %q", edge.Target)
		}
	}
	return nil
}

func renderProjectionSVG(projection Projection, nodes []ProjectionNode, edges []ProjectionEdge, positions map[string]svgPoint) []byte {
	width := svgMargin*2 + svgColumns*svgNodeWidth + (svgColumns-1)*svgGapX
	rows := int(math.Ceil(float64(max(len(nodes), 1)) / svgColumns))
	height := svgMargin*2 + 45 + rows*svgNodeHeight + max(rows-1, 0)*svgGapY
	var output bytes.Buffer
	fmt.Fprintf(&output, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", width, height, width, height)
	output.WriteString("  <defs><marker id=\"arrow\" markerWidth=\"8\" markerHeight=\"8\" refX=\"7\" refY=\"4\" orient=\"auto\"><path d=\"M 0 0 L 8 4 L 0 8 z\" fill=\"context-stroke\"/></marker></defs>\n")
	fmt.Fprintf(&output, "  <title>%s architecture projection</title>\n", escapeXML(projection.Kind))
	for _, edge := range edges {
		renderSVGEdge(&output, edge, positions)
	}
	for _, node := range nodes {
		renderSVGNode(&output, node, positions[node.ID])
	}
	output.WriteString("</svg>\n")
	return output.Bytes()
}

func renderSVGEdge(output *bytes.Buffer, edge ProjectionEdge, positions map[string]svgPoint) {
	source, target := positions[edge.Source], positions[edge.Target]
	path := svgEdgePath(source, target, edge.Source == edge.Target, edge.Status)
	color, dash := svgEdgeStyle(edge.Status)
	title := edge.Source + " -> " + edge.Target + " (" + string(edge.Status) + ")"
	if len(edge.ViolationKeys) > 0 {
		title += ": " + strings.Join(edge.ViolationKeys, ", ")
	}
	fmt.Fprintf(output, "  <path d=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"2\"%s marker-end=\"url(#arrow)\"><title>%s</title></path>\n", path, color, dash, escapeXML(title))
}

func svgEdgePath(source, target svgPoint, self bool, status ProjectionStatus) string {
	offset := svgStatusOffset(status)
	sx, sy := source.x+svgNodeWidth/2+offset, source.y+svgNodeHeight/2
	if self {
		return fmt.Sprintf("M %.0f %.0f C %.0f %.0f %.0f %.0f %.0f %.0f", sx, source.y, sx+55, source.y-42, sx-55, source.y-42, sx, source.y)
	}
	tx, ty := target.x+svgNodeWidth/2+offset, target.y+svgNodeHeight/2
	return fmt.Sprintf("M %.0f %.0f L %.0f %.0f", sx, sy, tx, ty)
}

func svgStatusOffset(status ProjectionStatus) float64 {
	switch status {
	case ProjectionAllowed:
		return -6
	case ProjectionNew:
		return 6
	default:
		return 0
	}
}

func svgEdgeStyle(status ProjectionStatus) (string, string) {
	switch status {
	case ProjectionLegacy:
		return "#d65a31", ""
	case ProjectionNew:
		return "#d00000", ` stroke-dasharray="8 6"`
	default:
		return "#737373", ""
	}
}

func renderSVGNode(output *bytes.Buffer, node ProjectionNode, point svgPoint) {
	title := node.ID + " (" + node.Kind + ")"
	if len(node.Packages) > 0 {
		title += ": " + strings.Join(node.Packages, ", ")
	}
	fmt.Fprintf(output, "  <g><title>%s</title><rect x=\"%.0f\" y=\"%.0f\" width=\"%d\" height=\"%d\" rx=\"7\" fill=\"#ffffff\" stroke=\"#222222\"/><text x=\"%.0f\" y=\"%.0f\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"14\">%s</text><text x=\"%.0f\" y=\"%.0f\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"11\" fill=\"#555555\">%s</text></g>\n",
		escapeXML(title), point.x, point.y, svgNodeWidth, svgNodeHeight, point.x+svgNodeWidth/2, point.y+23, escapeXML(node.ID), point.x+svgNodeWidth/2, point.y+41, escapeXML(node.Kind))
}

func escapeXML(value string) string {
	var output bytes.Buffer
	_ = xml.EscapeText(&output, []byte(value))
	return output.String()
}
