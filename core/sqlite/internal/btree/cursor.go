package btree

import (
	"fmt"
)

// Cursor state constants
const (
	CursorValid       = 0 // Cursor points to a valid entry
	CursorInvalid     = 1 // Cursor does not point to a valid entry
	CursorSkipNext    = 2 // Next/Previous should be a no-op
	CursorRequireSeek = 3 // Cursor position needs to be restored
	CursorFault       = 4 // Unrecoverable error
)

// Maximum B-tree depth (to prevent infinite loops in corrupt databases)
const MaxBtreeDepth = 20

// BtCursor represents a cursor for traversing a B-tree
type BtCursor struct {
	Btree    *Btree    // The B-tree this cursor belongs to
	RootPage uint32    // Root page number of the tree
	State    int       // Cursor state (valid, invalid, etc.)

	// Current position in the tree
	PageStack   [MaxBtreeDepth]uint32      // Stack of page numbers from root to current
	IndexStack  [MaxBtreeDepth]int         // Stack of cell indices
	Depth       int                        // Current depth in tree (0 = root)

	// Current cell information
	CurrentPage   uint32     // Current page number
	CurrentIndex  int        // Current cell index in page
	CurrentCell   *CellInfo  // Parsed current cell
	CurrentHeader *PageHeader // Current page header

	// Navigation flags
	AtFirst bool // True if at first entry
	AtLast  bool // True if at last entry
}

// NewCursor creates a new cursor for the given B-tree and root page
func NewCursor(bt *Btree, rootPage uint32) *BtCursor {
	return &BtCursor{
		Btree:    bt,
		RootPage: rootPage,
		State:    CursorInvalid,
		Depth:    -1,
	}
}

// MoveToFirst moves the cursor to the first entry in the B-tree
func (c *BtCursor) MoveToFirst() error {
	// Reset cursor state
	c.Depth = 0
	c.PageStack[0] = c.RootPage
	c.IndexStack[0] = 0
	c.AtFirst = false
	c.AtLast = false

	// Navigate to leftmost leaf
	pageNum := c.RootPage
	for {
		// Get page
		pageData, err := c.Btree.GetPage(pageNum)
		if err != nil {
			c.State = CursorInvalid
			return fmt.Errorf("failed to get page %d: %w", pageNum, err)
		}

		// Parse header
		header, err := ParsePageHeader(pageData, pageNum)
		if err != nil {
			c.State = CursorInvalid
			return fmt.Errorf("failed to parse page %d: %w", pageNum, err)
		}

		// Check if this is a leaf
		if header.IsLeaf {
			// We've reached a leaf - position at first cell
			if header.NumCells == 0 {
				c.State = CursorInvalid
				return fmt.Errorf("empty leaf page %d", pageNum)
			}

			c.CurrentPage = pageNum
			c.CurrentIndex = 0
			c.CurrentHeader = header
			c.AtFirst = true

			// Parse the first cell
			cellOffset, err := header.GetCellPointer(pageData, 0)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return err
			}
			c.CurrentCell = cell
			c.State = CursorValid
			return nil
		}

		// Interior page - follow first child pointer
		if header.NumCells == 0 {
			c.State = CursorInvalid
			return fmt.Errorf("empty interior page %d", pageNum)
		}

		// Get first cell to extract child page
		cellOffset, err := header.GetCellPointer(pageData, 0)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		// Navigate to child
		c.Depth++
		if c.Depth >= MaxBtreeDepth {
			c.State = CursorInvalid
			return fmt.Errorf("btree depth exceeded (possible corruption)")
		}

		pageNum = cell.ChildPage
		c.PageStack[c.Depth] = pageNum
		c.IndexStack[c.Depth] = 0
	}
}

// MoveToLast moves the cursor to the last entry in the B-tree
func (c *BtCursor) MoveToLast() error {
	// Reset cursor state
	c.Depth = 0
	c.PageStack[0] = c.RootPage
	c.AtFirst = false
	c.AtLast = false

	// Navigate to rightmost leaf
	pageNum := c.RootPage
	for {
		// Get page
		pageData, err := c.Btree.GetPage(pageNum)
		if err != nil {
			c.State = CursorInvalid
			return fmt.Errorf("failed to get page %d: %w", pageNum, err)
		}

		// Parse header
		header, err := ParsePageHeader(pageData, pageNum)
		if err != nil {
			c.State = CursorInvalid
			return fmt.Errorf("failed to parse page %d: %w", pageNum, err)
		}

		// Check if this is a leaf
		if header.IsLeaf {
			// We've reached a leaf - position at last cell
			if header.NumCells == 0 {
				c.State = CursorInvalid
				return fmt.Errorf("empty leaf page %d", pageNum)
			}

			c.CurrentPage = pageNum
			c.CurrentIndex = int(header.NumCells) - 1
			c.CurrentHeader = header
			c.AtLast = true
			c.IndexStack[c.Depth] = c.CurrentIndex

			// Parse the last cell
			cellOffset, err := header.GetCellPointer(pageData, c.CurrentIndex)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return err
			}
			c.CurrentCell = cell
			c.State = CursorValid
			return nil
		}

		// Interior page - follow rightmost child pointer
		// For interior pages, the rightmost child is in the header
		if header.RightChild == 0 {
			c.State = CursorInvalid
			return fmt.Errorf("interior page %d has no right child", pageNum)
		}

		// Navigate to rightmost child
		c.Depth++
		if c.Depth >= MaxBtreeDepth {
			c.State = CursorInvalid
			return fmt.Errorf("btree depth exceeded (possible corruption)")
		}

		pageNum = header.RightChild
		c.PageStack[c.Depth] = pageNum
		c.IndexStack[c.Depth] = -1 // Will be set when we reach the leaf
	}
}

// Next moves the cursor to the next entry
func (c *BtCursor) Next() error {
	if c.State != CursorValid {
		return fmt.Errorf("cursor not in valid state")
	}

	c.AtFirst = false

	// Get current page
	pageData, err := c.Btree.GetPage(c.CurrentPage)
	if err != nil {
		c.State = CursorInvalid
		return err
	}

	// If not at last cell in this page, just increment index
	if c.CurrentIndex < int(c.CurrentHeader.NumCells)-1 {
		c.CurrentIndex++
		c.IndexStack[c.Depth] = c.CurrentIndex

		// Parse next cell
		cellOffset, err := c.CurrentHeader.GetCellPointer(pageData, c.CurrentIndex)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		cell, err := ParseCell(c.CurrentHeader.PageType, pageData[cellOffset:], c.Btree.UsableSize)
		if err != nil {
			c.State = CursorInvalid
			return err
		}
		c.CurrentCell = cell
		return nil
	}

	// At last cell in page - need to go up the tree
	for c.Depth > 0 {
		c.Depth--
		parentPage := c.PageStack[c.Depth]
		parentIndex := c.IndexStack[c.Depth]

		parentData, err := c.Btree.GetPage(parentPage)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		parentHeader, err := ParsePageHeader(parentData, parentPage)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		// If not at last cell in parent, move to next cell in parent
		if parentIndex < int(parentHeader.NumCells)-1 {
			// Move to next cell in parent, then descend to first entry in that subtree
			c.IndexStack[c.Depth] = parentIndex + 1

			// Get the cell to find the child page
			cellOffset, err := parentHeader.GetCellPointer(parentData, parentIndex+1)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			cell, err := ParseCell(parentHeader.PageType, parentData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			// Descend to leftmost entry in this subtree
			return c.descendToFirst(cell.ChildPage)
		}
	}

	// Reached end of tree
	c.State = CursorInvalid
	c.AtLast = true
	return fmt.Errorf("end of btree")
}

// Previous moves the cursor to the previous entry
func (c *BtCursor) Previous() error {
	if c.State != CursorValid {
		return fmt.Errorf("cursor not in valid state")
	}

	c.AtLast = false

	// If not at first cell in this page, just decrement index
	if c.CurrentIndex > 0 {
		c.CurrentIndex--
		c.IndexStack[c.Depth] = c.CurrentIndex

		// Get current page
		pageData, err := c.Btree.GetPage(c.CurrentPage)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		// Parse previous cell
		cellOffset, err := c.CurrentHeader.GetCellPointer(pageData, c.CurrentIndex)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		cell, err := ParseCell(c.CurrentHeader.PageType, pageData[cellOffset:], c.Btree.UsableSize)
		if err != nil {
			c.State = CursorInvalid
			return err
		}
		c.CurrentCell = cell
		return nil
	}

	// At first cell in page - need to go up the tree
	for c.Depth > 0 {
		c.Depth--
		parentPage := c.PageStack[c.Depth]
		parentIndex := c.IndexStack[c.Depth]

		// If not at first cell in parent, move to previous cell in parent
		if parentIndex > 0 {
			c.IndexStack[c.Depth] = parentIndex - 1

			parentData, err := c.Btree.GetPage(parentPage)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			parentHeader, err := ParsePageHeader(parentData, parentPage)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			// Get the cell to find the child page
			cellOffset, err := parentHeader.GetCellPointer(parentData, parentIndex-1)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			cell, err := ParseCell(parentHeader.PageType, parentData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			// Descend to rightmost entry in this subtree
			return c.descendToLast(cell.ChildPage)
		}
	}

	// Reached beginning of tree
	c.State = CursorInvalid
	c.AtFirst = true
	return fmt.Errorf("beginning of btree")
}

// descendToFirst descends to the first (leftmost) entry starting from the given page
func (c *BtCursor) descendToFirst(pageNum uint32) error {
	for {
		c.Depth++
		if c.Depth >= MaxBtreeDepth {
			c.State = CursorInvalid
			return fmt.Errorf("btree depth exceeded")
		}

		c.PageStack[c.Depth] = pageNum
		c.IndexStack[c.Depth] = 0

		pageData, err := c.Btree.GetPage(pageNum)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		header, err := ParsePageHeader(pageData, pageNum)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		if header.IsLeaf {
			// Reached leaf
			if header.NumCells == 0 {
				c.State = CursorInvalid
				return fmt.Errorf("empty leaf")
			}

			c.CurrentPage = pageNum
			c.CurrentIndex = 0
			c.CurrentHeader = header

			cellOffset, err := header.GetCellPointer(pageData, 0)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return err
			}
			c.CurrentCell = cell
			c.State = CursorValid
			return nil
		}

		// Get first child
		cellOffset, err := header.GetCellPointer(pageData, 0)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		pageNum = cell.ChildPage
	}
}

// descendToLast descends to the last (rightmost) entry starting from the given page
func (c *BtCursor) descendToLast(pageNum uint32) error {
	for {
		c.Depth++
		if c.Depth >= MaxBtreeDepth {
			c.State = CursorInvalid
			return fmt.Errorf("btree depth exceeded")
		}

		c.PageStack[c.Depth] = pageNum

		pageData, err := c.Btree.GetPage(pageNum)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		header, err := ParsePageHeader(pageData, pageNum)
		if err != nil {
			c.State = CursorInvalid
			return err
		}

		if header.IsLeaf {
			// Reached leaf
			if header.NumCells == 0 {
				c.State = CursorInvalid
				return fmt.Errorf("empty leaf")
			}

			c.CurrentPage = pageNum
			c.CurrentIndex = int(header.NumCells) - 1
			c.CurrentHeader = header
			c.IndexStack[c.Depth] = c.CurrentIndex

			cellOffset, err := header.GetCellPointer(pageData, c.CurrentIndex)
			if err != nil {
				c.State = CursorInvalid
				return err
			}

			cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return err
			}
			c.CurrentCell = cell
			c.State = CursorValid
			return nil
		}

		// Follow rightmost child
		c.IndexStack[c.Depth] = int(header.NumCells)
		pageNum = header.RightChild
	}
}

// IsValid returns true if the cursor is pointing to a valid entry
func (c *BtCursor) IsValid() bool {
	return c.State == CursorValid
}

// GetKey returns the key of the current entry
func (c *BtCursor) GetKey() int64 {
	if c.State != CursorValid || c.CurrentCell == nil {
		return 0
	}
	return c.CurrentCell.Key
}

// GetPayload returns the payload of the current entry
func (c *BtCursor) GetPayload() []byte {
	if c.State != CursorValid || c.CurrentCell == nil {
		return nil
	}
	return c.CurrentCell.Payload
}

// String returns a string representation of the cursor
func (c *BtCursor) String() string {
	if c.State != CursorValid {
		return fmt.Sprintf("BtCursor{state=%d, invalid}", c.State)
	}
	return fmt.Sprintf("BtCursor{page=%d, index=%d, key=%d, depth=%d}",
		c.CurrentPage, c.CurrentIndex, c.GetKey(), c.Depth)
}

// SeekRowid seeks to the specified rowid in the table
// Returns true if the exact rowid is found, false otherwise
func (c *BtCursor) SeekRowid(rowid int64) (found bool, err error) {
	// Start from root
	c.Depth = 0
	c.PageStack[0] = c.RootPage
	c.IndexStack[0] = 0

	pageNum := c.RootPage

	// Navigate down the tree
	for {
		pageData, err := c.Btree.GetPage(pageNum)
		if err != nil {
			c.State = CursorInvalid
			return false, fmt.Errorf("failed to get page %d: %w", pageNum, err)
		}

		header, err := ParsePageHeader(pageData, pageNum)
		if err != nil {
			c.State = CursorInvalid
			return false, fmt.Errorf("failed to parse page %d: %w", pageNum, err)
		}

		// Binary search for the rowid
		idx, exactMatch := c.binarySearch(pageData, header, rowid)

		if header.IsLeaf {
			// Found the leaf page
			c.CurrentPage = pageNum
			c.CurrentIndex = idx
			c.CurrentHeader = header
			c.IndexStack[c.Depth] = idx

			if exactMatch && idx < int(header.NumCells) {
				// Parse the cell
				cellOffset, err := header.GetCellPointer(pageData, idx)
				if err != nil {
					c.State = CursorInvalid
					return false, err
				}

				cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
				if err != nil {
					c.State = CursorInvalid
					return false, err
				}

				c.CurrentCell = cell
				c.State = CursorValid
				return true, nil
			}

			// Rowid not found, but cursor is positioned
			c.State = CursorValid
			if idx < int(header.NumCells) {
				cellOffset, err := header.GetCellPointer(pageData, idx)
				if err == nil {
					cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
					if err == nil {
						c.CurrentCell = cell
					}
				}
			}
			return false, nil
		}

		// Interior page - follow the appropriate child
		var childPage uint32
		if idx >= int(header.NumCells) {
			// Follow right child
			childPage = header.RightChild
		} else {
			// Get cell to extract child page
			cellOffset, err := header.GetCellPointer(pageData, idx)
			if err != nil {
				c.State = CursorInvalid
				return false, err
			}

			cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
			if err != nil {
				c.State = CursorInvalid
				return false, err
			}

			childPage = cell.ChildPage
		}

		// Navigate to child
		c.Depth++
		if c.Depth >= MaxBtreeDepth {
			c.State = CursorInvalid
			return false, fmt.Errorf("btree depth exceeded")
		}

		pageNum = childPage
		c.PageStack[c.Depth] = pageNum
		c.IndexStack[c.Depth] = 0
	}
}

// binarySearch performs binary search for a rowid in a page
// Returns (index, exactMatch) where index is the position where the rowid should be
func (c *BtCursor) binarySearch(pageData []byte, header *PageHeader, rowid int64) (int, bool) {
	left := 0
	right := int(header.NumCells)

	for left < right {
		mid := (left + right) / 2

		// Get cell at mid
		cellOffset, err := header.GetCellPointer(pageData, mid)
		if err != nil {
			return left, false
		}

		cell, err := ParseCell(header.PageType, pageData[cellOffset:], c.Btree.UsableSize)
		if err != nil {
			return left, false
		}

		if cell.Key == rowid {
			return mid, true
		} else if cell.Key < rowid {
			left = mid + 1
		} else {
			right = mid
		}
	}

	return left, false
}

// Insert inserts a new row with the given key and payload
func (c *BtCursor) Insert(key int64, payload []byte) error {
	// Seek to the position where this key should be inserted
	found, err := c.SeekRowid(key)
	if err != nil {
		return err
	}

	if found {
		return fmt.Errorf("duplicate key: %d", key)
	}

	// We're now positioned at a leaf page
	if c.CurrentHeader == nil || !c.CurrentHeader.IsLeaf {
		return fmt.Errorf("cursor not positioned at leaf page")
	}

	// Encode the cell
	cellData := EncodeTableLeafCell(key, payload)

	// Get the current page
	pageData, err := c.Btree.GetPage(c.CurrentPage)
	if err != nil {
		return err
	}

	// Wrap in BtreePage for write operations
	btreePage, err := NewBtreePage(c.CurrentPage, pageData, c.Btree.UsableSize)
	if err != nil {
		return err
	}

	// Check if the cell will fit on the page
	if len(cellData) > btreePage.FreeSpace() {
		// Page is full - need to split
		return c.splitPage(key, payload)
	}

	// Insert the cell
	if err := btreePage.InsertCell(c.CurrentIndex, cellData); err != nil {
		return err
	}

	// Update the cursor to point to the newly inserted cell
	if _, err := c.SeekRowid(key); err != nil {
		return err
	}

	return nil
}

// Delete deletes the row at the current cursor position
func (c *BtCursor) Delete() error {
	if c.State != CursorValid {
		return fmt.Errorf("cursor not in valid state")
	}

	if c.CurrentHeader == nil || !c.CurrentHeader.IsLeaf {
		return fmt.Errorf("cursor not positioned at leaf page")
	}

	// Get the current page
	pageData, err := c.Btree.GetPage(c.CurrentPage)
	if err != nil {
		return err
	}

	// Wrap in BtreePage for write operations
	btreePage, err := NewBtreePage(c.CurrentPage, pageData, c.Btree.UsableSize)
	if err != nil {
		return err
	}

	// Delete the cell
	if err := btreePage.DeleteCell(c.CurrentIndex); err != nil {
		return err
	}

	// Invalidate cursor
	c.State = CursorInvalid

	return nil
}

// splitPage splits a full page when inserting a new cell
// This is a simplified implementation - a full implementation would need to:
// 1. Allocate a new page
// 2. Distribute cells between old and new page
// 3. Update parent page (or create new root if splitting root)
// 4. Handle propagation of splits up the tree
func (c *BtCursor) splitPage(key int64, payload []byte) error {
	// For now, return an error indicating the page needs to be split
	// A full implementation would handle the split here
	return fmt.Errorf("page split not yet implemented (page %d is full)", c.CurrentPage)
}
