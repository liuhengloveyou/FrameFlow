package vision

// filterBoxesByMinConf 按置信度过滤。
func filterBoxesByMinConf(boxes []VisionBBox, minConf float32) []VisionBBox {
	if minConf <= 0 || len(boxes) == 0 {
		return boxes
	}
	out := make([]VisionBBox, 0, len(boxes))
	for _, b := range boxes {
		if float32(b.Score) >= minConf {
			out = append(out, b)
		}
	}
	return out
}

// filterBoxesByRegions 保留中心点落在任一多边形内的框；regions 空则原样返回。
func filterBoxesByRegions(boxes []VisionBBox, regions [][]NormPoint) []VisionBBox {
	if len(regions) == 0 || len(boxes) == 0 {
		return boxes
	}
	out := make([]VisionBBox, 0, len(boxes))
	for _, b := range boxes {
		cx := b.X + b.W*0.5
		cy := b.Y + b.H*0.5
		for _, poly := range regions {
			if pointInPolygon(cx, cy, poly) {
				out = append(out, b)
				break
			}
		}
	}
	return out
}

func pointInPolygon(x, y float64, poly []NormPoint) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	j := len(poly) - 1
	for i := 0; i < len(poly); i++ {
		xi, yi := poly[i].X, poly[i].Y
		xj, yj := poly[j].X, poly[j].Y
		intersect := ((yi > y) != (yj > y)) &&
			(x < (xj-xi)*(y-yi)/(yj-yi+1e-12)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}

// applyStreamPolicy 在检测结果上应用 ROI（置信度已在 YOLO 后处理按 policy.Conf 过滤，此处不再二次过滤）。
func applyStreamPolicy(boxes []VisionBBox, p StreamAIPolicy) []VisionBBox {
	if !p.Enabled {
		return nil
	}
	return filterBoxesByRegions(boxes, p.Regions)
}
