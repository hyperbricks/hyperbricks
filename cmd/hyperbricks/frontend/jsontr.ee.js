/**
 * https://github.com/xzitlou/jsontr.ee - MIT License
 * Generates an SVG visualization of a JSON object as a tree.
 */

function generateJSONTree(json) {
    // Basic layout configuration
    const padding = 12; // Internal spacing of nodes
    const lineHeight = 18; // Line height for text within nodes
    const fontSize = 14; // Font size
    const fontFamily = "monospace"; // Font family for text
    let svgContent = []; // Stores SVG elements representing nodes
    let edges = []; // Stores lines connecting nodes
    let nodeId = 0; // Counter to assign a unique ID to each node
    let maxX = 0; // Maximum X coordinate of the SVG
    let maxY = 0; // Maximum Y coordinate of the SVG
    const occupiedPositions = []; // Tracks occupied positions to avoid overlapping

    /**
     * Generates a unique base hue for each new branch.
     */
    let baseHue = 0;
    function generateBaseHue() {
        const hue = baseHue;
        baseHue = (baseHue + 90) % 360; // Increment hue by 90 degrees for more distinct colors
        return hue;
    }

    /**
     * Generates a color based on the base hue and level.
     * @param {number} baseHue - The base hue for the branch.
     * @param {number} level - The current level in the branch.
     * @returns {string} - Color in HSL format.
     */
    function generateColor(baseHue, level) {
        const saturation = 65 + (level * 5); // Increase saturation with depth for vibrancy
        const lightness = 50 - (level * 5); // Decrease lightness with depth
        // Ensure saturation and lightness remain within 0-100%
        const adjustedSaturation = Math.min(100, Math.max(30, saturation));
        const adjustedLightness = Math.min(90, Math.max(20, lightness));
        return `hsl(${baseHue}, ${adjustedSaturation}%, ${adjustedLightness}%)`;
    }

    /**
     * Measures the width of a text based on font settings.
     * @param {string} text - Text to measure.
     * @returns {number} - Width of the text in pixels.
     */
    function measureTextWidth(text) {
        const canvas = document.createElement("canvas");
        const context = canvas.getContext("2d");
        context.font = `${fontSize}px ${fontFamily}`;
        return context.measureText(text).width;
    }

    /**
     * Calculates the size of a node based on its content.
     * @param {Object|Array|string|number|null} obj - JSON object or value to visualize.
     * @returns {Object} - Dimensions (width, height) and text lines of the node.
     */
    function calculateNodeSize(obj) {
        const lines = []; // Stores text lines of the node

        // Determine text lines based on data type
        if (Array.isArray(obj)) {
            lines.push({ key: "", value: `Array (${obj.length})` });
        } else if (typeof obj === "object" && obj !== null) {
            for (const [key, value] of Object.entries(obj)) {
                const displayValue = Array.isArray(value)
                    ? `Array (${value.length})`
                    : typeof value === "object"
                        ? "{}"
                        : JSON.stringify(value);
                lines.push({ key, value: escapeHTMLUsingDOM(displayValue).replace(/\\n/g, '').replace(/\\"/g, '"') });
            }
        } else {
            lines.push({ key: "", value: JSON.stringify(obj) });
        }

        // Calculate node width and height based on text lines
        const maxWidth = Math.max(...lines.map(line => measureTextWidth(`${line.key}: ${line.value}`)));
        const height = lines.length * lineHeight + padding * 2;

        return { width: maxWidth + padding * 2, height, lines };
    }

    /**
     * Adjusts the position of a node to avoid overlapping with other nodes.
     * @param {number} x - Initial X coordinate.
     * @param {number} y - Initial Y coordinate.
     * @param {number} width - Width of the node.
     * @param {number} height - Height of the node.
     * @returns {number} - Adjusted Y coordinate.
     */
    function adjustPosition(x, y, width, height) {
        let adjustedY = y;
        const buffer = 10; // Spacing between nodes to prevent collisions

        for (const pos of occupiedPositions) {
            const [ox, oy, ow, oh] = pos;
            if (
                x < ox + ow &&
                x + width > ox &&
                adjustedY < oy + oh &&
                adjustedY + height > oy
            ) {
                adjustedY = oy + oh + buffer; // Adjust downward if there's a collision
            }
        }

        // Register the position as occupied
        occupiedPositions.push([x, adjustedY, width, height]);

        return adjustedY;
    }

    /**
     * Escapes HTML characters using the DOM.
     * @param {string} input - Text to escape.
     * @returns {string} - Escaped text.
     */
    function escapeHTMLUsingDOM(input) {
        const div = document.createElement('div');
        div.textContent = input;
        return div.innerHTML;
    }

    /**
     * Recursively builds the tree from the JSON and generates nodes and connections.
     * @param {Object} obj - JSON object or value to visualize.
     * @param {number} x - X coordinate of the current node.
     * @param {number} y - Y coordinate of the current node.
     * @param {string|null} parentId - ID of the parent node (if any).
     * @param {Object|null} parentPosition - Position of the parent node (if any).
     * @param {number|null} parentBaseHue - Base hue of the parent branch (if any).
     * @param {number} level - Current level in the branch (root is level 0).
     */
    function buildTree(obj, x, y, parentId = null, parentPosition = null, parentBaseHue = null, level = 0) {
        const { width, height, lines } = calculateNodeSize(obj);
        const adjustedY = adjustPosition(x, y, width, height);
        const currentId = `node-${nodeId++}`; // Unique ID for the current node

        // Determine the base hue
        let currentBaseHue;
        if (level === 0) {
            // For the root node, assign a default base hue (e.g., 0 for red)
            currentBaseHue = generateBaseHue();
        } else if (level === 1) {
            // For first-level children, generate new base hues for each branch
            currentBaseHue = generateBaseHue();
        } else {
            // For deeper levels, inherit the base hue from the parent
            currentBaseHue = parentBaseHue;
        }

        // Determine the color based on the base hue and level
        const currentColor = generateColor(currentBaseHue, level);

        // Generate the node content using flexbox for alignment
        const nodeContent = lines
            .map(line => `
                <div style="display: flex;">
                    <span class="json-key" style="margin-right: 5px;">${line.key ? `${line.key}:` : ""}</span>
                    <span class="json-value">${line.value}</span>
                </div>
            `)
            .join("");

        // Check if the current object has ContentType or @type equal to PAGE and has a slug
        const hasContentTypePage = obj["ContentType"]  == '<WEBPAGE>' || obj['@type'] == '<WEBPAGE>' 

        const slug = obj.Slug;

        // Generate the node's SVG group
        let nodeGroup = `
            <g id="${currentId}" transform="translate(${x}, ${adjustedY})">
                <rect width="${width}" height="${height}" rx="5" ry="5" style="fill:#dcdcdc;stroke:${currentColor};stroke-width:2;"></rect>
                <foreignObject width="${width}" height="${height}">
                    <div xmlns="http://www.w3.org/1999/xhtml" style="font-family:${fontFamily}; font-size:${fontSize}px; line-height:${lineHeight}px; padding:${padding}px; box-sizing:border-box;">
                        ${nodeContent}
                    </div>
                </foreignObject>
            </g>
        `;

        // If the node should be clickable, wrap it in an <a> element
        if (hasContentTypePage) {
            const url = `/statusviewer/${encodeURIComponent(slug)}`;
            nodeGroup = `
                <a href="${url}" target="_self">
                    ${nodeGroup}
                </a>
            `;
        }

        // Add the node (or linked node) to the SVG content
        svgContent.push(nodeGroup);

        // If the node has a parent, draw a connection (curved line) with the parent's color
        if (parentId && parentPosition && parentBaseHue !== null) {
            const parentCenterX = parentPosition.x + parentPosition.width;
            const parentCenterY = parentPosition.y + parentPosition.height / 2;
            const childCenterX = x;
            const childCenterY = adjustedY + height / 2;

            const parentColor = generateColor(parentBaseHue, level - 1);

            edges.push(`
                <path d="M${parentCenterX},${parentCenterY} C${(parentCenterX + childCenterX) / 2},${parentCenterY} ${(parentCenterX + childCenterX) / 2},${childCenterY} ${childCenterX},${childCenterY}"
                      style="fill:none;stroke:${parentColor};stroke-width:2;" />
            `);
        }

        let nextYOffset = adjustedY;

        // Process the children of the current node
        lines.forEach((line, index) => {
            const value = obj[line.key];
            const childX = x + width + 100;

            if (Array.isArray(value)) {
                const listNode = { [`${line.key} (${value.length})`]: "Array" };
                buildTree(
                    listNode,
                    childX,
                    nextYOffset,
                    currentId,
                    { x, y: adjustedY, width, height },
                    currentBaseHue,
                    level + 1
                );

                value.forEach((item, idx) => {
                    const childY = nextYOffset + idx * (lineHeight + 30);
                    buildTree(
                        item,
                        childX + calculateNodeSize(listNode).width + 100,
                        childY,
                        `node-${nodeId - 1}`,
                        {
                            x: childX,
                            y: nextYOffset,
                            width: calculateNodeSize(listNode).width,
                            height: calculateNodeSize(listNode).height,
                        },
                        currentBaseHue,
                        level + 2
                    );
                });

                nextYOffset += value.length * (lineHeight + 30) + 50;
            } else if (typeof value === "object" && value !== null) {
                buildTree(
                    value,
                    childX,
                    nextYOffset,
                    currentId,
                    { x, y: adjustedY, width, height },
                    currentBaseHue,
                    level + 1
                );
                nextYOffset += calculateNodeSize(value).height + 50;
            }
        });

        maxX = Math.max(maxX, x + width);
        maxY = Math.max(maxY, nextYOffset);
    }

    // Start building the tree from the root node
    buildTree(json, 50, 50);

    // Generate the final SVG
    return `
        <svg xmlns="http://www.w3.org/2000/svg" width="${maxX + 150}" height="${maxY + 150}">
            <defs>
                <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto">
                    <polygon points="0 0, 10 3.5, 0 7" style="fill:#475872;" />
                </marker>
            </defs>
            ${edges.join("")}
            ${svgContent.join("")}
        </svg>
    `;
}
