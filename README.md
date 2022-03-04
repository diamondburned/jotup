# jotup

A prototype Markdown editor in GTK4. Nothing works yet.

## Diffing Algorithm

0. Assign current `ast.Node` tree to the rendered widget tree.
1. Render Markdown text to a new `ast.Node` tree.
2. Diff the `ast.Node` trees:
	- For each level, the code should be able to traverse from the top-level
	  node down to the widget.
	- The widget gets popped off, and a new tree is allocated.
	- Recursively repeat for each level.
