package rating

import (
	"fmt"
	"sort"

	"github.com/alpacahq/alpacadecimal"
	"github.com/samber/lo"

	"github.com/openmeterio/openmeter/openmeter/billing/charges/usagebased"
	"github.com/openmeterio/openmeter/openmeter/billing/models/totals"
)

type RatedRunDetails struct {
	Totals        totals.Totals
	DetailedLines usagebased.DetailedLines
}

func SubtractRatedRunDetails(a, b RatedRunDetails) (RatedRunDetails, error) {
	if err := a.DetailedLines.Validate(); err != nil {
		return RatedRunDetails{}, fmt.Errorf("a detailed lines: %w", err)
	}

	if err := b.DetailedLines.Validate(); err != nil {
		return RatedRunDetails{}, fmt.Errorf("b detailed lines: %w", err)
	}

	aByReferenceID, err := aggregateDetailedLinesByReferenceID(a.DetailedLines)
	if err != nil {
		return RatedRunDetails{}, fmt.Errorf("aggregate a detailed lines: %w", err)
	}

	bByReferenceID, err := aggregateDetailedLinesByReferenceID(b.DetailedLines)
	if err != nil {
		return RatedRunDetails{}, fmt.Errorf("aggregate b detailed lines: %w", err)
	}

	detailedLines := subtractDetailedLinesByReferenceID(aByReferenceID, bByReferenceID)

	return RatedRunDetails{
		Totals:        sumDetailedLineTotals(detailedLines),
		DetailedLines: detailedLines,
	}, nil
}

func aggregateDetailedLinesByReferenceID(lines usagebased.DetailedLines) (map[string]usagebased.DetailedLine, error) {
	grouped := make(map[string][]usagebased.DetailedLine, len(lines))

	for _, line := range lines {
		parsed, err := parseChildUniqueReferenceID(line.ChildUniqueReferenceID)
		if err != nil {
			return nil, err
		}

		grouped[parsed.ReferenceID] = append(grouped[parsed.ReferenceID], line)
	}

	return lo.MapValues(grouped, func(group []usagebased.DetailedLine, _ string) usagebased.DetailedLine {
		return sumDetailedLines(group)
	}), nil
}

func sumDetailedLines(lines []usagebased.DetailedLine) usagebased.DetailedLine {
	line := lines[0].Clone()
	line.ChildUniqueReferenceID = lo.Must(parseChildUniqueReferenceID(line.ChildUniqueReferenceID)).ReferenceID
	line.Quantity = alpacadecimal.Zero
	line.Totals = totals.Totals{}

	for _, item := range lines {
		line.Quantity = line.Quantity.Add(item.Quantity)
		line.Totals = line.Totals.Add(item.Totals)
	}

	return line
}

func subtractDetailedLinesByReferenceID(a, b map[string]usagebased.DetailedLine) usagebased.DetailedLines {
	keys := lo.Union(lo.Keys(a), lo.Keys(b))
	sort.Strings(keys)

	out := make(usagebased.DetailedLines, 0, len(keys))

	for _, key := range keys {
		line, ok := subtractDetailedLine(a[key], b[key])
		if !ok {
			continue
		}

		out = append(out, line)
	}

	return out
}

func subtractDetailedLine(a, b usagebased.DetailedLine) (usagebased.DetailedLine, bool) {
	switch {
	case a.ChildUniqueReferenceID == "":
		line := b.Clone()
		line.Quantity = line.Quantity.Neg()
		line.Totals = line.Totals.Neg()
		return line, !isZeroDetailedLine(line)
	case b.ChildUniqueReferenceID == "":
		line := a.Clone()
		return line, !isZeroDetailedLine(line)
	default:
		line := a.Clone()
		line.Quantity = line.Quantity.Sub(b.Quantity)
		line.Totals = line.Totals.Sub(b.Totals)
		return line, !isZeroDetailedLine(line)
	}
}

func isZeroDetailedLine(line usagebased.DetailedLine) bool {
	return line.Totals == (totals.Totals{})
}

func sumDetailedLineTotals(lines usagebased.DetailedLines) totals.Totals {
	out := totals.Totals{}

	for _, line := range lines {
		out = out.Add(line.Totals)
	}

	return out
}
