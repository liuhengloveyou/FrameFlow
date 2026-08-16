package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

func handleWorkflowsList(w http.ResponseWriter, r *http.Request) {
	items, err := aiBizBackend.ListWorkflows()
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrServer.SetMsg(err.Error()))
		return
	}
	ResponseData(w, items)
}

func handleWorkflowCreate(w http.ResponseWriter, r *http.Request) {
	var in WorkflowCreateInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	item, err := aiBizBackend.CreateWorkflow(in)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleWorkflowNodeSpecs(w http.ResponseWriter, r *http.Request) {
	ResponseData(w, aiBizBackend.ListNodeSpecs())
}

func handleWorkflowNotifyLatest(w http.ResponseWriter, r *http.Request) {
	item, seq := aiBizBackend.LatestWorkflowToast()
	ResponseData(w, map[string]any{
		"seq":  seq,
		"item": item,
	})
}

func handleWorkflowGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := aiBizBackend.GetWorkflow(id)
	if err != nil {
		fferrors.RespondBizError(w, fferrors.ErrNotFound.SetMsg(err.Error()))
		return
	}
	ResponseData(w, item)
}

func handleWorkflowUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var raw map[string]json.RawMessage
	if err := DecodeJSON(r, &raw); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	var name, desc *string
	if v, ok := raw["name"]; ok {
		var s string
		_ = json.Unmarshal(v, &s)
		name = &s
	}
	if v, ok := raw["description"]; ok {
		var s string
		_ = json.Unmarshal(v, &s)
		desc = &s
	}
	var nodes []WorkflowNode
	var edges []WorkflowEdge
	hasNodes, hasEdges := false, false
	if v, ok := raw["nodes"]; ok {
		hasNodes = true
		_ = json.Unmarshal(v, &nodes)
	}
	if v, ok := raw["edges"]; ok {
		hasEdges = true
		_ = json.Unmarshal(v, &edges)
	}
	item, err := aiBizBackend.UpdateWorkflow(id, name, desc, nodes, edges, hasNodes, hasEdges)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleWorkflowDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := aiBizBackend.DeleteWorkflow(id); err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, map[string]string{"status": "ok"})
}

func handleWorkflowStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := aiBizBackend.StartWorkflow(id)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleWorkflowStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := aiBizBackend.StopWorkflow(id)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, item)
}

func handleWorkflowDebug(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in WorkflowDebugInput
	if err := DecodeJSON(r, &in); err != nil {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg(err.Error()))
		return
	}
	if in.InputMode == "" {
		in.InputMode = "latest_alarm"
	}
	if in.ReturnMode == "" {
		in.ReturnMode = "sync"
	}
	runID, events, err := aiBizBackend.DebugWorkflow(id, in)
	if err != nil {
		fferrors.RespondErr(w, err)
		return
	}
	ResponseData(w, map[string]any{
		"run_id": runID,
		"events": events,
	})
}

func handleWorkflowDebugStream(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		fferrors.RespondBizError(w, fferrors.ErrBadRequest.SetMsg("missing run_id"))
		return
	}
	// 调试事件已在 DebugWorkflow 时缓冲；此处做 SSE 回放
	type streamer interface {
		DebugStream(runID string, send func(DebugEvent) error) error
	}
	st, ok := aiBizBackend.(streamer)
	if !ok {
		// 回退：空流结束
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprintf(w, "event: done\ndata: {}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)
	err := st.DebugStream(runID, func(ev DebugEvent) error {
		b, _ := json.Marshal(ev)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(30 * time.Millisecond) // 便于前端逐条高亮
		return nil
	})
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
	}
	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}
