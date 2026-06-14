package libvirt

import (
	"fmt"
	"regexp"
	"strings"

	golibvirt "github.com/digitalocean/go-libvirt"
)

func updateDomainBootDisk(l *golibvirt.Libvirt, domain golibvirt.Domain, volPath, diskFormat, poolName, volName string) (golibvirt.Domain, error) {
	xmlDesc, err := l.DomainGetXMLDesc(domain, 0)
	if err != nil {
		return golibvirt.Domain{}, fmt.Errorf("get domain xml: %w", err)
	}

	updated, err := patchDomainBootDiskXML(xmlDesc, volPath, diskFormat, poolName, volName)
	if err != nil {
		return golibvirt.Domain{}, err
	}

	defined, err := l.DomainDefineXML(updated)
	if err != nil {
		return golibvirt.Domain{}, fmt.Errorf("update domain boot disk: %w", err)
	}

	return defined, nil
}

var domainDiskPattern = regexp.MustCompile(`(?s)<disk\b[^>]*>.*?</disk>`)

func patchDomainBootDiskXML(xmlDesc, volPath, diskFormat, poolName, volName string) (string, error) {
	matches := domainDiskPattern.FindAllStringSubmatchIndex(xmlDesc, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("domain has no disk devices")
	}

	selected := selectBootDiskIndex(matches, xmlDesc)
	if selected < 0 {
		return "", fmt.Errorf("domain has no boot disk candidate")
	}

	loc := matches[selected]
	updatedBlock, err := patchDiskBlock(xmlDesc[loc[0]:loc[1]], volPath, diskFormat, poolName, volName)
	if err != nil {
		return "", err
	}

	return xmlDesc[:loc[0]] + updatedBlock + xmlDesc[loc[1]:], nil
}

func selectBootDiskIndex(matches [][]int, xmlDesc string) int {
	selected := -1
	selectedOrder := 0

	for i, loc := range matches {
		block := xmlDesc[loc[0]:loc[1]]
		if !isBootDiskCandidate(block) {
			continue
		}

		order := diskBootOrder(block)
		if selected < 0 {
			selected = i
			selectedOrder = order
			if order == 1 {
				return selected
			}
			continue
		}
		if order == 1 {
			return i
		}
		if selectedOrder == 1 {
			continue
		}
		if order > 0 && (selectedOrder == 0 || order < selectedOrder) {
			selected = i
			selectedOrder = order
		}
	}

	return selected
}

func isBootDiskCandidate(diskXML string) bool {
	device := attributeValue(diskXML, "device")
	if device == "" {
		return true
	}
	switch strings.ToLower(device) {
	case "disk":
		return true
	case "cdrom", "floppy":
		return false
	default:
		return false
	}
}

func diskBootOrder(diskXML string) int {
	bootTag := regexp.MustCompile(`(?s)<boot\b[^>]*order=['"](\d+)['"]`).FindStringSubmatch(diskXML)
	if len(bootTag) == 2 {
		var order int
		_, _ = fmt.Sscanf(bootTag[1], "%d", &order)
		return order
	}
	return 0
}

func patchDiskBlock(diskXML, volPath, diskFormat, poolName, volName string) (string, error) {
	openTagPattern := regexp.MustCompile(`(?s)^(<disk\b[^>]*>)`)
	match := openTagPattern.FindStringSubmatch(diskXML)
	if len(match) != 2 {
		return "", fmt.Errorf("invalid disk xml")
	}

	updated := setAttribute(match[1], "type", "volume") + diskXML[len(match[1]):]
	updated = replaceOrInsertDriver(updated, diskFormat)
	updated = replaceOrInsertSource(updated, volPath, poolName, volName)
	return updated, nil
}

func replaceOrInsertDriver(diskXML, diskFormat string) string {
	driverPattern := regexp.MustCompile(`(?s)<driver\b[^>]*/>`)
	replacement := fmt.Sprintf(`<driver name='qemu' type='%s'/>`, xmlAttr(diskFormat))
	if driverPattern.MatchString(diskXML) {
		return driverPattern.ReplaceAllString(diskXML, replacement)
	}

	openTag := regexp.MustCompile(`(?s)^(<disk\b[^>]*>)`).FindStringSubmatch(diskXML)
	if len(openTag) == 2 {
		return openTag[1] + "\n      " + replacement + diskXML[len(openTag[1]):]
	}
	return diskXML
}

func replaceOrInsertSource(diskXML, volPath, poolName, volName string) string {
	sourcePattern := regexp.MustCompile(`(?s)<source\b[^>]*/>`)
	var replacement string
	if poolName != "" && volName != "" {
		replacement = fmt.Sprintf(
			`<source pool='%s' volume='%s'/>`,
			xmlAttr(poolName),
			xmlAttr(volName),
		)
	} else {
		replacement = fmt.Sprintf(`<source file='%s'/>`, xmlAttr(volPath))
	}
	if sourcePattern.MatchString(diskXML) {
		return sourcePattern.ReplaceAllString(diskXML, replacement)
	}

	driverPattern := regexp.MustCompile(`(?s)<driver\b[^>]*/>`)
	if loc := driverPattern.FindStringIndex(diskXML); loc != nil {
		return diskXML[:loc[1]] + "\n      " + replacement + diskXML[loc[1]:]
	}

	openTag := regexp.MustCompile(`(?s)^(<disk\b[^>]*>)`).FindStringSubmatch(diskXML)
	if len(openTag) == 2 {
		return openTag[1] + "\n      " + replacement + diskXML[len(openTag[1]):]
	}

	return diskXML
}

func attributeValue(tag, name string) string {
	pattern := regexp.MustCompile(fmt.Sprintf(`\b%s=['"]([^'"]*)['"]`, regexp.QuoteMeta(name)))
	match := pattern.FindStringSubmatch(tag)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func setAttribute(tag, name, value string) string {
	escapedName := regexp.QuoteMeta(name)

	singleQuoted := regexp.MustCompile(fmt.Sprintf(`(\b%s=)'[^']*'`, escapedName))
	if singleQuoted.MatchString(tag) {
		return singleQuoted.ReplaceAllString(tag, fmt.Sprintf(`$1'%s'`, xmlAttr(value)))
	}

	doubleQuoted := regexp.MustCompile(fmt.Sprintf(`(\b%s=)"[^"]*"`, escapedName))
	if doubleQuoted.MatchString(tag) {
		return doubleQuoted.ReplaceAllString(tag, fmt.Sprintf(`$1"%s"`, xmlAttr(value)))
	}

	open := regexp.MustCompile(`^(<\w+\b)([^>]*)(/?>)`).FindStringSubmatch(tag)
	if len(open) != 4 {
		return tag
	}
	attrs := strings.TrimSpace(open[2])
	if attrs == "" {
		return open[1] + fmt.Sprintf(` %s='%s'`, name, xmlAttr(value)) + open[3] + tag[len(open[0]):]
	}
	return open[1] + open[2] + fmt.Sprintf(` %s='%s'`, name, xmlAttr(value)) + open[3] + tag[len(open[0]):]
}

func xmlAttr(value string) string {
	return strings.NewReplacer(`&`, "&amp;", `'`, "&apos;", `"`, "&quot;", `<`, "&lt;", `>`, "&gt;").Replace(value)
}
