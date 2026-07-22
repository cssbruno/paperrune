// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

type ProvenanceID uint32

func (id ProvenanceID) Valid() bool { return id != 0 }

// ProvenanceEntry is one interned identity/source tuple. Canonical order is
// deterministic first occurrence across fragments followed by line sources.
type ProvenanceEntry struct {
	Node     NodeID     `json:"node"`
	Key      NodeKey    `json:"key"`
	Instance InstanceID `json:"instance"`
	Source   SourceSpan `json:"source"`
}

func buildCompactProvenance(fragments []Fragment, lines []PlannedLine) ([]ProvenanceEntry, []ProvenanceID, []ProvenanceID) {
	table := make([]ProvenanceEntry, 0, len(fragments))
	var maxNode NodeID
	for _, fragment := range fragments {
		if fragment.Node > maxNode {
			maxNode = fragment.Node
		}
	}
	var denseNodeIDs []ProvenanceID
	var sparseNodeIDs map[NodeID]ProvenanceID
	if uint64(maxNode) <= uint64(len(fragments))*4+1 {
		denseNodeIDs = make([]ProvenanceID, int(maxNode)+1)
	} else {
		sparseNodeIDs = make(map[NodeID]ProvenanceID, len(fragments))
	}
	var variants map[NodeID][]ProvenanceID
	firstForNode := func(node NodeID) ProvenanceID {
		if denseNodeIDs != nil {
			return denseNodeIDs[node]
		}
		return sparseNodeIDs[node]
	}
	setFirstForNode := func(node NodeID, id ProvenanceID) {
		if denseNodeIDs != nil {
			denseNodeIDs[node] = id
		} else {
			sparseNodeIDs[node] = id
		}
	}
	intern := func(entry ProvenanceEntry) ProvenanceID {
		if first := firstForNode(entry.Node); first.Valid() {
			if table[first-1] == entry {
				return first
			}
			for _, id := range variants[entry.Node] {
				if table[id-1] == entry {
					return id
				}
			}
		}
		id := ProvenanceID(len(table) + 1) // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
		table = append(table, entry)
		if firstForNode(entry.Node).Valid() {
			if variants == nil {
				variants = make(map[NodeID][]ProvenanceID)
			}
			variants[entry.Node] = append(variants[entry.Node], id)
		} else {
			setFirstForNode(entry.Node, id)
		}
		return id
	}
	fragmentRefs := make([]ProvenanceID, len(fragments))
	denseFragmentIDs := true
	var sparseFragmentRefs map[FragmentID]ProvenanceID
	for index, fragment := range fragments {
		fragmentRefs[index] = intern(ProvenanceEntry{Node: fragment.Node, Key: fragment.Key, Instance: fragment.Instance, Source: fragment.Source})
		if fragment.ID != FragmentID(index+1) {
			denseFragmentIDs = false
		}
	}
	if !denseFragmentIDs {
		sparseFragmentRefs = make(map[FragmentID]ProvenanceID, len(fragments))
		for index, fragment := range fragments {
			sparseFragmentRefs[fragment.ID] = fragmentRefs[index]
		}
	}
	lineRefs := make([]ProvenanceID, len(lines))
	for index, line := range lines {
		var fragmentRef ProvenanceID
		if denseFragmentIDs {
			fragmentRef = fragmentRefs[line.Fragment-1]
		} else {
			fragmentRef = sparseFragmentRefs[line.Fragment]
		}
		fragmentEntry := table[fragmentRef-1]
		fragmentEntry.Source = line.Source
		lineRefs[index] = intern(fragmentEntry)
	}
	return table, fragmentRefs, lineRefs
}

// ResolveProvenance returns a detached compact-table entry.
func (p LayoutPlan) ResolveProvenance(id ProvenanceID) (ProvenanceEntry, bool) {
	if !id.Valid() || uint64(id) > uint64(len(p.provenance)) {
		return ProvenanceEntry{}, false
	}
	return p.provenance[id-1], true
}
