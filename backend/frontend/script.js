"use strict";

const fileInput = document.getElementById("fileInput");
const fileText = document.getElementById("fileText");
const uploadForm = document.getElementById("uploadForm");
const fileList = document.getElementById("fileList");

// ---------------------------------------------------------------------------
// RELOAD GUARD — warn before closing/refreshing while uploads are in progress
// ---------------------------------------------------------------------------
let uploadsInProgress = 0;

window.addEventListener("beforeunload", (e) => {
  if (uploadsInProgress > 0) {
    // Modern browsers show their own generic message; setting returnValue
    // is still required to trigger the dialog.
    e.preventDefault();
    e.returnValue = "Files are still uploading. If you leave, the upload will be cancelled.";
    return e.returnValue;
  }
});

// 1. Update text when files are selected
fileInput.addEventListener("change", function () {
  if (fileInput.files.length > 0) {
    const count = fileInput.files.length;
    fileText.textContent = count === 1 ? fileInput.files[0].name : `${count} files selected`;
    fileText.style.color = "#111827";
  } else {
    fileText.textContent = "Select files...";
    fileText.style.color = "";
  }
});

// ---------------------------------------------------------------------------
// 2. Parallel Upload Engine
// ---------------------------------------------------------------------------
const MAX_CONCURRENT = 4; // upload up to 4 files simultaneously

/**
 * Upload a single File via XHR so we get upload progress events.
 * Returns a Promise that resolves when the upload completes.
 */
function uploadFile(file, onProgress) {
  uploadsInProgress++;
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();

    xhr.upload.addEventListener("progress", (e) => {
      if (e.lengthComputable) onProgress(e.loaded / e.total);
    });

    xhr.addEventListener("load", () => {
      uploadsInProgress--;
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress(1);
        resolve();
      } else {
        reject(new Error(`Server returned ${xhr.status}`));
      }
    });

    xhr.addEventListener("error", () => { uploadsInProgress--; reject(new Error("Network error")); });
    xhr.addEventListener("abort", () => { uploadsInProgress--; reject(new Error("Upload aborted")); });

    const fd = new FormData();
    fd.append("files", file);
    xhr.open("POST", "/upload");
    xhr.send(fd);
  });
}

/** Format bytes to a human-readable string */
function formatBytes(bytes) {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

/** Create a progress row element for a single file */
function createProgressRow(file) {
  const row = document.createElement("div");
  row.className = "upload-progress-row";

  const nameEl = document.createElement("span");
  nameEl.className = "upload-progress-name";
  nameEl.textContent = file.name.length > 28 ? file.name.slice(0, 25) + "…" : file.name;

  const sizeEl = document.createElement("span");
  sizeEl.className = "upload-progress-size";
  sizeEl.textContent = formatBytes(file.size);

  const track = document.createElement("div");
  track.className = "upload-progress-track";

  const fill = document.createElement("div");
  fill.className = "upload-progress-fill";
  fill.style.width = "0%";
  track.appendChild(fill);

  const pct = document.createElement("span");
  pct.className = "upload-progress-pct";
  pct.textContent = "0%";

  const meta = document.createElement("div");
  meta.className = "upload-progress-meta";
  meta.appendChild(nameEl);
  meta.appendChild(sizeEl);

  row.appendChild(meta);
  row.appendChild(track);
  row.appendChild(pct);

  const update = (ratio) => {
    const p = Math.round(ratio * 100);
    fill.style.width = p + "%";
    pct.textContent = p + "%";
    if (p >= 100) {
      fill.classList.add("upload-progress-done");
      pct.textContent = "✓";
    }
  };

  return { row, update };
}

// 3. Handle Upload
uploadForm.addEventListener("submit", async function (e) {
  e.preventDefault();
  if (!fileInput.files.length) return;

  const files = Array.from(fileInput.files);
  const btn = uploadForm.querySelector("button");
  btn.textContent = "Uploading…";
  btn.disabled = true;

  // Build progress UI
  let progressContainer = document.getElementById("uploadProgressContainer");
  if (!progressContainer) {
    progressContainer = document.createElement("div");
    progressContainer.id = "uploadProgressContainer";
    progressContainer.className = "upload-progress-container";
    uploadForm.insertAdjacentElement("afterend", progressContainer);
  }
  progressContainer.innerHTML = "";

  const progressRows = files.map((file) => {
    const { row, update } = createProgressRow(file);
    progressContainer.appendChild(row);
    return { file, update };
  });

  // Concurrency-limited parallel upload queue
  const queue = [...progressRows];

  async function worker() {
    while (queue.length) {
      const { file, update } = queue.shift();
      try {
        await uploadFile(file, update);
      } catch (err) {
        console.error("Upload failed for", file.name, err);
      }
    }
  }

  // Spawn up to MAX_CONCURRENT workers
  const workers = Array.from(
    { length: Math.min(MAX_CONCURRENT, files.length) },
    () => worker()
  );

  try {
    await Promise.all(workers);
    // Reset form
    fileInput.value = "";
    fileText.textContent = "Select files...";
    fileText.style.color = "";
    loadFiles();
  } catch (err) {
    console.error("Upload batch error:", err);
    alert("Some files failed to upload.");
  } finally {
    btn.textContent = "Upload";
    btn.disabled = false;
    // Clear progress UI after a short delay
    setTimeout(() => {
      if (progressContainer) progressContainer.innerHTML = "";
    }, 2000);
  }
});


// 3. Load and Render Files
function loadFiles() {
  fetch("/files")
    .then((res) => res.json())
    .then((files) => {
      fileList.innerHTML = "";

      if (files.length === 0) {
        fileList.innerHTML = `<div style="text-align:center; color:#4b5563; font-size:12px; margin-top:20px;">No files shared yet</div>`;
        return;
      }

      files.forEach((file) => {
        const row = document.createElement("div");
        row.className = "file-card";

        // Name
        const nameSpan = document.createElement("span");
        nameSpan.className = "file-name";
        nameSpan.textContent = file.name;

        // Actions Container
        const actionsDiv = document.createElement("div");
        actionsDiv.className = "file-actions";

        // Preview Button
        const previewBtn = document.createElement("button");
        previewBtn.className = "icon-btn preview-btn";
        previewBtn.innerHTML = "&#128065;"; // eye emoji
        previewBtn.title = "Preview";
        previewBtn.onclick = () => openPreview(file.name);

        const dlBtn = document.createElement("button");
        dlBtn.className = "icon-btn";
        dlBtn.innerHTML = "\u2b07"; // Clear arrow icon
        dlBtn.title = "Download";
        dlBtn.onclick = () => {
          window.location = "/download?name=" + encodeURIComponent(file.name);
        };

        // Delete Button
        const delBtn = document.createElement("button");
        delBtn.className = "icon-btn delete-btn";
        delBtn.innerHTML = "\u2715"; // Clear X icon
        delBtn.title = "Delete";
        delBtn.onclick = () => {
          if (confirm(`Delete "${file.name}"?`)) {
            deleteFile(file.name);
          }
        };

        actionsDiv.appendChild(previewBtn);
        actionsDiv.appendChild(dlBtn);
        actionsDiv.appendChild(delBtn);

        row.appendChild(nameSpan);
        row.appendChild(actionsDiv);

        fileList.appendChild(row);
      });
    })
    .catch((err) => console.error("Load error:", err));
}

function deleteFile(filename) {
  fetch("/delete?name=" + encodeURIComponent(filename), {
    method: "DELETE",
  })
    .then(() => loadFiles())
    .catch((err) => alert("Delete failed"));
}

function renderFiles(fileList) {
  const grid = document.getElementById("file-grid");
  const template = document.getElementById("file-card-template");

  // Clear current list
  grid.innerHTML = "";

  fileList.forEach((file) => {
    // Clone the template
    const clone = template.content.cloneNode(true);

    // Select elements inside the clone
    const img = clone.querySelector(".file-thumb");
    const name = clone.querySelector(".file-name");
    const card = clone.querySelector(".file-card");

    // Set Name
    name.textContent = file.name;

    // Set Thumbnail Logic
    if (isImageFile(file.name)) {
      // POINT TO OUR NEW GO ROUTE
      // Encodes the path component to handle spaces/special chars safely
      img.src = `/thumbnail/${encodeURIComponent(file.path)}`;
    } else {
      // Fallback for non-images (PDFs, EXEs, etc.)
      img.src = "/assets/icons/file_generic.svg";
      // Or add a class to style it differently
      img.style.objectFit = "contain";
      img.style.padding = "20px";
    }

    // Add Click Event (Download or Open)
    card.onclick = () => {
      window.location.href = `/download/${file.path}`;
    };

    // Append to grid
    grid.appendChild(clone);
  });
}

function isImageFile(filename) {
  const ext = filename.split(".").pop().toLowerCase();
  return ["jpg", "jpeg", "png", "gif", "webp"].includes(ext);
}

// =========================================
//  FILE PREVIEW MODAL
// =========================================
const previewModal = document.getElementById("previewModal");
const previewBody = document.getElementById("previewBody");
const previewFilename = document.getElementById("previewFilename");
const previewDownloadBtn = document.getElementById("previewDownloadBtn");
const previewCloseBtn = document.getElementById("previewCloseBtn");
const previewBackdrop = document.getElementById("previewBackdrop");

function getFileType(name) {
  const ext = name.split(".").pop().toLowerCase();
  if (["jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "ico"].includes(ext)) return "image";
  if (["mp4", "webm", "mov", "mkv", "avi"].includes(ext)) return "video";
  if (["mp3", "wav", "ogg", "flac", "aac", "m4a"].includes(ext)) return "audio";
  if (ext === "pdf") return "pdf";
  if (["txt", "md", "json", "csv", "xml", "yaml", "yml", "log", "js", "ts", "css", "html", "sh", "py", "go", "rs"].includes(ext)) return "text";
  return "other";
}

function openPreview(filename) {
  const url = "/download?name=" + encodeURIComponent(filename);
  const type = getFileType(filename);

  previewFilename.textContent = filename;
  previewDownloadBtn.href = url;
  previewDownloadBtn.download = filename;
  previewBody.innerHTML = "";

  let content;

  if (type === "image") {
    content = document.createElement("img");
    content.src = url;
    content.alt = filename;
    content.className = "preview-img";
  } else if (type === "video") {
    content = document.createElement("video");
    content.src = url;
    content.controls = true;
    content.autoplay = false;
    content.className = "preview-video";
  } else if (type === "audio") {
    const wrapper = document.createElement("div");
    wrapper.className = "preview-audio-wrapper";
    const icon = document.createElement("div");
    icon.className = "preview-audio-icon";
    icon.textContent = "🎵";
    content = document.createElement("audio");
    content.src = url;
    content.controls = true;
    content.className = "preview-audio";
    wrapper.appendChild(icon);
    wrapper.appendChild(content);
    previewBody.appendChild(wrapper);
    previewModal.classList.add("open");
    document.body.style.overflow = "hidden";
    return;
  } else if (type === "pdf") {
    content = document.createElement("iframe");
    content.src = url;
    content.className = "preview-iframe";
    content.title = filename;
  } else if (type === "text") {
    content = document.createElement("div");
    content.className = "preview-text-loading";
    content.textContent = "Loading…";
    previewBody.appendChild(content);
    previewModal.classList.add("open");
    document.body.style.overflow = "hidden";
    fetch(url)
      .then((r) => r.text())
      .then((text) => {
        const pre = document.createElement("pre");
        pre.className = "preview-text";
        pre.textContent = text;
        previewBody.innerHTML = "";
        previewBody.appendChild(pre);
      })
      .catch(() => {
        previewBody.innerHTML =
          '<div class="preview-unsupported">Could not load text file.</div>';
      });
    return;
  } else {
    content = document.createElement("div");
    content.className = "preview-unsupported";
    content.innerHTML = `<span class="preview-file-icon">📄</span><p>No preview available for this file type.</p><a href="${url}" download="${filename}" class="preview-action-btn">⬇ Download</a>`;
  }

  previewBody.appendChild(content);
  previewModal.classList.add("open");
  document.body.style.overflow = "hidden";
}

function closePreview() {
  previewModal.classList.remove("open");
  document.body.style.overflow = "";
  // Stop any media playback
  previewBody.querySelectorAll("video, audio").forEach((el) => {
    el.pause();
    el.src = "";
  });
  previewBody.innerHTML = "";
}

previewCloseBtn.addEventListener("click", closePreview);
previewBackdrop.addEventListener("click", closePreview);
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") closePreview();
});

// Initial Load
loadFiles();
