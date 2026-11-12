package ui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// logo is the three hexagons of the ocm.software icon as line art. Like in the icon
// they are pointy-topped and every pair shares an edge, meeting in the center.
var logo = []string{
	`    _.--'--._    `,
	`    |       |    `,
	`    |       |    `,
	`_.--'--._.--'--._`,
	`|       |       |`,
	`|       |       |`,
	`'--._.--'--._.--'`,
}

// LogoHeight is the number of lines Logo renders.
const LogoHeight = 7

// Logo renders the line-art logo, shaded top to bottom along the brand gradient.
var Logo = sync.OnceValue(func() string {
	from, _ := colorful.Hex(GradientFrom)
	to, _ := colorful.Hex(GradientTo)
	lines := make([]string, len(logo))
	for i, line := range logo {
		c := from.BlendLuv(to, float64(i)/float64(len(logo)-1))
		lines[i] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Hex())).Render(line)
	}
	return strings.Join(lines, "\n")
})
