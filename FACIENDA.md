# FACIENDA - Things to be done

## Phase 2: Session UI (Simple)
- [ ] bubbletea TUI with scrolling output (read-only)
- [ ] Dedicated input line at bottom
- [ ] Input history (up/down)
- [ ] Proper APL character handling

## Phase 2b: Session UI (Full)
- [ ] Evolve to single editable session buffer
- [ ] Navigate anywhere, edit previous inputs, re-execute
- [ ] Multiline input handling (RIDE does this poorly - opportunity to improve)

## Phase 3: Editors
- [ ] Handle OpenWindow/UpdateWindow
- [ ] Text editing for functions/operators
- [ ] SaveChanges

## Phase 4: Tracer
- [ ] Debug UI
- [ ] Stepping, breakpoints

## Phase 5: Dialogs
- [ ] OptionsDialog (yes/no/cancel)
- [ ] StringDialog (text input)

---

## Notes

### Session Behavior
The Dyalog session is append-only from the interpreter's perspective. Client shows editable history, but executing always appends:
1. User sees previous input `1+1` with result `2`
2. User navigates up, edits to `1+2`, executes
3. Original line resets to `1+1`
4. New line `1+2` appended, then result `3`

### Multiline Editing
RIDE handles multiline poorly. Research needed on:
- How interpreter expects multiline input
- What protocol messages are involved
- Opportunity to do better than RIDE

---

## Completed

### Phase 1: Minimal RIDE Client
- [x] Connect to Dyalog/multiplexer
- [x] Implement handshake
- [x] Execute APL code and display output
