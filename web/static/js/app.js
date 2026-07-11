"use strict";

const uploadForm = document.getElementById("upload-form");
const fileInput = document.getElementById("file-input");
const uploadStatus = document.getElementById("upload-status");

const stepConfigure = document.getElementById("step-configure");
const sourceInfo = document.getElementById("source-info");
const jobForm = document.getElementById("job-form");
const outputFormatSelect = document.getElementById("output-format");

const stepProgress = document.getElementById("step-progress");
const progressFill = document.getElementById("progress-fill");
const progressText = document.getElementById("progress-text");
const cancelBtn = document.getElementById("cancel-btn");

const stepDone = document.getElementById("step-done");
const doneText = document.getElementById("done-text");
const downloadLink = document.getElementById("download-link");

let state = { uploadId: null, kind: null, jobId: null, eventSource: null };

uploadForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const file = fileInput.files[0];
  if (!file) return;

  uploadStatus.textContent = "Uploading and probing…";
  const body = new FormData();
  body.append("file", file);

  try {
    const res = await fetch("/api/v1/uploads", { method: "POST", body });
    const data = await res.json();
    if (!res.ok) {
      uploadStatus.textContent = describeError(data);
      return;
    }
    state.uploadId = data.upload_id;
    state.kind = data.info.kind;
    uploadStatus.textContent = `Uploaded "${data.filename}".`;
    renderSourceInfo(data.info);
    await populateOutputFormats(data.info.kind);
    stepConfigure.classList.remove("hidden");
  } catch (err) {
    uploadStatus.textContent = "Upload failed: " + err.message;
  }
});

function renderSourceInfo(info) {
  const rows = [
    ["Kind", info.kind],
    ["Container", info.format],
    ["Duration", info.duration_s ? `${info.duration_s.toFixed(2)}s` : "—"],
    ["Video codec", info.video_codec || "—"],
    ["Audio codec", info.audio_codec || "—"],
    ["Dimensions", info.width ? `${info.width}x${info.height}` : "—"],
    ["Bit rate", info.bit_rate ? `${Math.round(info.bit_rate / 1000)} kbps` : "—"],
  ];
  sourceInfo.innerHTML = rows
    .map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`)
    .join("");
}

async function populateOutputFormats(kind) {
  const res = await fetch("/api/v1/formats");
  const data = await res.json();
  const formats = data.output_formats[kind] || [];
  outputFormatSelect.innerHTML = formats
    .map((f) => `<option value="${f}">${f}</option>`)
    .join("");
}

jobForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    upload_id: state.uploadId,
    output_format: outputFormatSelect.value,
    width: numOrZero("width"),
    height: numOrZero("height"),
    frame_rate: numOrZero("frame-rate"),
    sample_rate: numOrZero("sample-rate"),
    bit_rate: document.getElementById("bit-rate").value.trim(),
  };

  const res = await fetch("/api/v1/jobs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json();
  if (!res.ok) {
    alert(describeError(data));
    return;
  }

  state.jobId = data.id;
  stepConfigure.classList.add("hidden");
  stepProgress.classList.remove("hidden");
  watchJob(data.id);
});

function numOrZero(id) {
  const v = document.getElementById(id).value;
  return v === "" ? 0 : Number(v);
}

function watchJob(jobId) {
  const es = new EventSource(`/api/v1/jobs/${jobId}/events`);
  state.eventSource = es;

  es.onmessage = (evt) => {
    const job = JSON.parse(evt.data);
    const pct = Math.max(0, Math.min(100, job.progress?.percent || 0));
    progressFill.style.width = pct.toFixed(1) + "%";

    switch (job.state) {
      case "pending":
        progressText.textContent = `Queued (position ${job.queue_position ?? 0})…`;
        break;
      case "running":
        progressText.textContent = `Encoding… ${pct.toFixed(1)}%`;
        break;
      case "completed":
        es.close();
        finish(job, `Done in ${elapsed(job)}.`, true);
        break;
      case "failed":
        es.close();
        finish(job, `Failed: ${job.error}`, false);
        break;
      case "cancelled":
        es.close();
        finish(job, "Job was cancelled.", false);
        break;
      case "timed_out":
        es.close();
        finish(job, "Job exceeded the maximum allowed duration.", false);
        break;
    }
  };
}

function elapsed(job) {
  if (!job.started_at || !job.completed_at) return "";
  const ms = new Date(job.completed_at) - new Date(job.started_at);
  return (ms / 1000).toFixed(1) + "s";
}

function finish(job, message, ok) {
  stepProgress.classList.add("hidden");
  stepDone.classList.remove("hidden");
  doneText.textContent = message;
  if (ok) {
    downloadLink.href = `/api/v1/jobs/${job.id}/download`;
    downloadLink.classList.remove("hidden");
  } else {
    downloadLink.classList.add("hidden");
  }
}

cancelBtn.addEventListener("click", async () => {
  if (!state.jobId) return;
  await fetch(`/api/v1/jobs/${state.jobId}`, { method: "DELETE" });
});

function describeError(data) {
  if (data && data.error) {
    let msg = data.error.message || "request failed";
    if (data.error.supported_input_extensions) {
      msg += " (" + data.error.supported_input_extensions.join(", ") + ")";
    }
    return msg;
  }
  return "request failed";
}
