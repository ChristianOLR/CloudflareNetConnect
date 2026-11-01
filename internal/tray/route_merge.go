package tray

import "github.com/chgarcia79/cloudflarenetconnect/internal/config"

func mergeRoutes(existing, discovered []config.TunnelRoute) []config.TunnelRoute {
	if len(existing) == 0 {
		return ensureUniquePorts(discovered)
	}
	if len(discovered) == 0 {
		return ensureUniquePorts(existing)
	}
	byHost := make(map[string]config.TunnelRoute)
	for _, r := range existing {
		byHost[r.Hostname] = r
	}
	var merged []config.TunnelRoute
	seen := make(map[string]bool)
	for _, d := range discovered {
		if ex, ok := byHost[d.Hostname]; ok {
			d.Enabled = ex.Enabled
			if ex.LocalPort > 0 {
				d.LocalPort = ex.LocalPort
			}
		}
		merged = append(merged, d)
		seen[d.Hostname] = true
	}
	for _, r := range existing {
		if !seen[r.Hostname] {
			merged = append(merged, r)
		}
	}
	return ensureUniquePorts(merged)
}

func ensureUniquePorts(routes []config.TunnelRoute) []config.TunnelRoute {
	used := make(map[int]bool)
	for i := range routes {
		if routes[i].LocalPort <= 0 {
			continue
		}
		for used[routes[i].LocalPort] {
			routes[i].LocalPort++
		}
		used[routes[i].LocalPort] = true
	}
	return routes
}
