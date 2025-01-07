package shared

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/emirpasic/gods/maps/treemap"
	"github.com/emirpasic/gods/utils"
)

var (
	sortedKeysCache = make(map[string][]string)
	cacheMu         sync.RWMutex
)

// sanitizeDuplicates removes duplicate strings from a slice.
func sanitizeDuplicates(input []string) []string {
	seen := make(map[string]bool) // To track seen elements
	result := []string{}

	for _, value := range input {
		if !seen[value] { // If the value hasn't been seen before
			seen[value] = true             // Mark it as seen
			result = append(result, value) // Add it to the result slice
		}
	}

	return result
}

// SortedKeys returns the keys of the map sorted numerically or lexicographically.
// It caches the sorted keys for maps with the same set of keys.
// This uses TreeMap, is 5 times faster and uses 53 times less memory than the non-TreeMap
func SortedUniqueKeys(m map[string]interface{}) []string {
	// Generate a unique identifier for the map based on its keys.
	var keyList []string
	for k := range m {
		if k == "@type" {
			continue
		}
		keyList = append(keyList, k)
	}

	// Sanitize keys to remove duplicates
	keyList = sanitizeDuplicates(keyList)

	sort.Strings(keyList) // Ensure consistent cacheKey generation
	cacheKey := strings.Join(keyList, "|")

	// Check if sorted keys are already cached.
	cacheMu.RLock()
	sorted, exists := sortedKeysCache[cacheKey]
	cacheMu.RUnlock()
	if exists {
		return sorted
	}

	// Use TreeMap to sort keys (numerically or lexicographically).
	treeMap := treemap.NewWith(func(a, b interface{}) int {
		keyA, keyB := a.(string), b.(string)
		intA, errA := strconv.Atoi(keyA)
		intB, errB := strconv.Atoi(keyB)

		// If both are integers, compare numerically.
		if errA == nil && errB == nil {
			return utils.IntComparator(intA, intB)
		}
		// Integers come before strings.
		if errA == nil {
			return -1
		}
		if errB == nil {
			return 1
		}
		// Otherwise, compare lexicographically.
		return strings.Compare(keyA, keyB)
	})

	// Insert all keys into the TreeMap.
	for _, k := range keyList {
		treeMap.Put(k, nil)
	}

	// Extract sorted keys.
	sorted = make([]string, 0, treeMap.Size())
	treeMap.Each(func(key, _ interface{}) {
		sorted = append(sorted, key.(string))
	})

	// Cache the sorted keys.
	cacheMu.Lock()
	sortedKeysCache[cacheKey] = sorted
	cacheMu.Unlock()

	return sorted
}

// TrimWrap removes the wrapping content around the placeholder '|'.
func TrimWrap(content, html string) string {
	start := strings.Index(html, "|")
	if start == -1 {
		return html
	}
	prefix := content[:strings.Index(content, "|")]
	suffix := content[strings.Index(content, "|")+1:]
	return prefix + html[start+1:] + suffix
}

// EncloseContent wraps the given content with the specified wrap string.
// The wrap string can be a single tag or a pair separated by '|'.
func EncloseContent(wrap string, content string) string {
	if wrap == "" {
		return content
	}

	// Split the wrap string into two parts using the first occurrence of '|'
	parts := strings.SplitN(wrap, "|", 2)

	if len(parts) == 2 {
		// Dual-Part Wrap: Use the first part as prefix and the second as suffix
		prefix := strings.TrimSpace(parts[0])
		suffix := strings.TrimSpace(parts[1])
		return fmt.Sprintf("%s%s%s", prefix, content, suffix)
	}

	// Single-Part Wrap: Assume it's an HTML tag and construct opening and closing tags
	tag := strings.TrimSpace(parts[0])

	// Check if the tag already includes angle brackets
	if strings.HasPrefix(tag, "<") && strings.HasSuffix(tag, ">") {
		// Extract the tag name (e.g., "<p>" -> "p")
		tagName := strings.Trim(tag, "<>/")
		return fmt.Sprintf("<%s>%s</%s>", tagName, content, tagName)
	}

	// If the tag does not include angle brackets, add them
	return fmt.Sprintf("<%s>%s</%s>", tag, content, tag)
}

func ToMap(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, true
	default:
		return nil, false
	}
}

// RenderAllowedAttributes filters and formats the allowed attributes for rendering in a consistent order.
func RenderAllowedAttributes(attributes map[string]interface{}, allowed []string) string {
	var attrBuilder strings.Builder

	// Iterate over the allowed attributes in order
	for _, key := range allowed {
		if value, exists := attributes[key]; exists {
			attrBuilder.WriteString(fmt.Sprintf(` %s="%s"`, key, value))
		}
	}

	return attrBuilder.String()
}
