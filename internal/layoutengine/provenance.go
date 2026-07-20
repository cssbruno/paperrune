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
	ids := make(map[ProvenanceEntry]ProvenanceID, len(fragments))
	intern := func(entry ProvenanceEntry) ProvenanceID {
		if id := ids[entry]; id.Valid() {
			return id
		}
		id := ProvenanceID(len(table) + 1) // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
		table = append(table, entry)
		ids[entry] = id
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
