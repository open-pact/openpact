package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/stack-bound/stackllm/agent"
	"github.com/stack-bound/stackllm/conversation"
)

// streamEvents builds a buffered channel pre-populated with the given
// events so accumulateChatResponse can drain it synchronously.
func streamEvents(evs ...agent.Event) <-chan agent.Event {
	ch := make(chan agent.Event, len(evs))
	for _, ev := range evs {
		ch <- ev
	}
	close(ch)
	return ch
}

func textBlockEvents(parts ...string) []agent.Event {
	evs := make([]agent.Event, 0, len(parts)*2)
	for _, p := range parts {
		blk := conversation.Block{Type: conversation.BlockText, Text: p}
		evs = append(evs,
			agent.Event{Type: agent.EventBlockDelta, BlockType: conversation.BlockText, Content: p},
			agent.Event{Type: agent.EventBlockEnd, Block: &blk},
		)
	}
	return evs
}

// Multiple text blocks from a single turn must all survive into the
// final response — prior implementation reset the buffer on each
// BlockEnd and only kept the last block.
func TestAccumulateMultipleTextBlocksPreservedWithSeparator(t *testing.T) {
	events := streamEvents(
		append(
			textBlockEvents("Looking that up...", "Here's what I found: 42."),
			agent.Event{Type: agent.EventComplete},
		)...,
	)

	resp, _, err := accumulateChatResponse(events, false, false)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	want := "Looking that up...\n\nHere's what I found: 42."
	if resp.Text != want {
		t.Fatalf("text = %q, want %q", resp.Text, want)
	}
}

// When the agent emits only a single text block, no separator is
// added — the separator is a join, not a trailing newline.
func TestAccumulateSingleTextBlockNoSeparator(t *testing.T) {
	events := streamEvents(
		append(
			textBlockEvents("Just one answer."),
			agent.Event{Type: agent.EventComplete},
		)...,
	)

	resp, _, err := accumulateChatResponse(events, false, false)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	if resp.Text != "Just one answer." {
		t.Fatalf("text = %q, want %q", resp.Text, "Just one answer.")
	}
}

// BlockDelta events are discarded in favour of the BlockEnd's final
// text — deltas are for progressive rendering, the final block is
// canonical. If the provider streamed "Hel" + "lo" but the final
// block is "Hello there!" we should see the final.
func TestAccumulateDeltasOverriddenByBlockEnd(t *testing.T) {
	finalBlk := conversation.Block{Type: conversation.BlockText, Text: "Hello there!"}
	events := streamEvents(
		agent.Event{Type: agent.EventBlockDelta, BlockType: conversation.BlockText, Content: "Hel"},
		agent.Event{Type: agent.EventBlockDelta, BlockType: conversation.BlockText, Content: "lo"},
		agent.Event{Type: agent.EventBlockEnd, Block: &finalBlk},
		agent.Event{Type: agent.EventComplete},
	)

	resp, _, err := accumulateChatResponse(events, false, false)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	if resp.Text != "Hello there!" {
		t.Fatalf("text = %q, want %q", resp.Text, "Hello there!")
	}
}

// If the stream ends with buffered deltas and no BlockEnd (e.g. the
// connection dropped mid-block) we flush the deltas so the user
// still sees partial text rather than a silently empty response.
func TestAccumulateFlushesOrphanedDeltas(t *testing.T) {
	events := streamEvents(
		agent.Event{Type: agent.EventBlockDelta, BlockType: conversation.BlockText, Content: "partial "},
		agent.Event{Type: agent.EventBlockDelta, BlockType: conversation.BlockText, Content: "message"},
	)

	resp, _, err := accumulateChatResponse(events, false, false)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	if resp.Text != "partial message" {
		t.Fatalf("text = %q, want %q", resp.Text, "partial message")
	}
}

// Text → tool_use → text is the canonical shape flagged in review.
// The pre-tool framing ("Looking that up...") must survive alongside
// the post-tool answer, plus the tool-call record when wantTools=true.
func TestAccumulateTextToolTextInterleave(t *testing.T) {
	pre := conversation.Block{Type: conversation.BlockText, Text: "Looking that up..."}
	toolUse := conversation.Block{Type: conversation.BlockToolUse, ToolName: "workspace_read", ToolArgsJSON: `{"path":"SOUL.md"}`}
	post := conversation.Block{Type: conversation.BlockText, Text: "Found it."}

	events := streamEvents(
		agent.Event{Type: agent.EventBlockEnd, Block: &pre},
		agent.Event{Type: agent.EventBlockEnd, Block: &toolUse},
		agent.Event{Type: agent.EventToolResult, ToolCall: &conversation.ToolCall{Name: "workspace_read"}, ToolResult: "file contents"},
		agent.Event{Type: agent.EventBlockEnd, Block: &post},
		agent.Event{Type: agent.EventComplete},
	)

	resp, _, err := accumulateChatResponse(events, false, true)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	want := "Looking that up...\n\nFound it."
	if resp.Text != want {
		t.Fatalf("text = %q, want %q", resp.Text, want)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "workspace_read" {
		t.Errorf("tool name = %q, want workspace_read", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Output != "file contents" {
		t.Errorf("tool output = %q, want %q", resp.ToolCalls[0].Output, "file contents")
	}
}

// wantThinking=false must drop thinking blocks entirely so they
// don't leak into bot responses running in simple/tools mode.
func TestAccumulateDropsThinkingWhenNotWanted(t *testing.T) {
	think := conversation.Block{Type: conversation.BlockThinking, Text: "internal monologue"}
	reply := conversation.Block{Type: conversation.BlockText, Text: "visible reply"}

	events := streamEvents(
		agent.Event{Type: agent.EventBlockEnd, Block: &think},
		agent.Event{Type: agent.EventBlockEnd, Block: &reply},
		agent.Event{Type: agent.EventComplete},
	)

	resp, _, err := accumulateChatResponse(events, false, false)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	if resp.Thinking != "" {
		t.Errorf("thinking leaked when wantThinking=false: %q", resp.Thinking)
	}
	if resp.Text != "visible reply" {
		t.Errorf("text = %q", resp.Text)
	}
}

// wantThinking=true: multiple thinking blocks concatenate with a
// blank line, same rule as text blocks.
func TestAccumulateThinkingPreservedAcrossBlocks(t *testing.T) {
	t1 := conversation.Block{Type: conversation.BlockThinking, Text: "first thought"}
	t2 := conversation.Block{Type: conversation.BlockThinking, Text: "second thought"}

	events := streamEvents(
		agent.Event{Type: agent.EventBlockEnd, Block: &t1},
		agent.Event{Type: agent.EventBlockEnd, Block: &t2},
		agent.Event{Type: agent.EventComplete},
	)

	resp, _, err := accumulateChatResponse(events, true, false)
	if err != nil {
		t.Fatalf("accumulateChatResponse: %v", err)
	}
	// The existing code strips literal "\n" sequences from thinking output.
	if !strings.Contains(resp.Thinking, "first thought") || !strings.Contains(resp.Thinking, "second thought") {
		t.Errorf("thinking missing a block: %q", resp.Thinking)
	}
}

// EventError should stop the loop and return the error; messages from
// the partial conversation still propagate for persistence.
func TestAccumulatePropagatesErrorAndMessages(t *testing.T) {
	boom := errors.New("provider exploded")
	partial := []conversation.Message{{Role: conversation.RoleAssistant}}

	events := streamEvents(
		agent.Event{Type: agent.EventError, Err: boom, Messages: partial},
	)

	resp, msgs, err := accumulateChatResponse(events, false, false)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if resp != nil {
		t.Errorf("resp should be nil on error, got %+v", resp)
	}
	if len(msgs) != 1 {
		t.Errorf("messages len = %d, want 1", len(msgs))
	}
}
