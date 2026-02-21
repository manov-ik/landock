"use strict";

const fileInput = document.getElementById("fileInput");
const fileText = document.getElementById("fileText");
const uploadForm = document.getElementById("uploadForm");
const fileList = document.getElementById("fileList");

// 1. Update text when files are selected
fileInput.addEventListener("change", function () {
  if (fileInput.files.length > 0) {
    const count = fileInput.files.length;
    if (count === 1) {
      fileText.textContent = fileInput.files[0].name;
    } else {
      fileText.textContent = `${count} files selected`;
    }
    fileText.style.color = "#111827"; // Use dark text for visibility
  } else {
    fileText.textContent = "Select files...";
    fileText.style.color = ""; // Reset color
  }
});

// 2. Handle Upload
uploadForm.addEventListener("submit", function (e) {
  e.preventDefault();

  if (!fileInput.files.length) return;

  const originalBtnText = uploadForm.querySelector("button").textContent;
  const btn = uploadForm.querySelector("button");

  // Show loading state
  btn.textContent = "Uploading...";
  btn.disabled = true;

  const data = new FormData();
  for (let i = 0; i < fileInput.files.length; i++) {
    data.append("files", fileInput.files[i]);
  }

  fetch("/upload", {
    method: "POST",
    body: data,
  })
    .then(function () {
      // Reset form
      fileInput.value = "";
      fileText.textContent = "Select files...";
      fileText.style.color = "";
      loadFiles();
    })
    .catch(function (err) {
      console.error("Upload error:", err);
      alert("Upload failed.");
    })
    .finally(function () {
      // Reset button
      btn.textContent = originalBtnText;
      btn.disabled = false;
    });
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

        const dlBtn = document.createElement("button");
        dlBtn.className = "icon-btn";
        dlBtn.innerHTML = "⬇"; // Clear arrow icon
        dlBtn.title = "Download";
        dlBtn.onclick = () => {
          window.location = "/download?name=" + encodeURIComponent(file.name);
        };

        // Delete Button
        const delBtn = document.createElement("button");
        delBtn.className = "icon-btn delete-btn";
        delBtn.innerHTML = "✕"; // Clear X icon
        delBtn.title = "Delete";
        delBtn.onclick = () => {
          if (confirm(`Delete "${file.name}"?`)) {
            deleteFile(file.name);
          }
        };

        // // Download Button
        // const dlBtn = document.createElement("button");
        // dlBtn.className = "icon-btn";
        // dlBtn.textContent = "⬇";
        // dlBtn.title = "Download";
        // dlBtn.onclick = () => {
        //   window.location = "/download?name=" + encodeURIComponent(file.name);
        // };

        // // Delete Button
        // const delBtn = document.createElement("button");
        // delBtn.className = "icon-btn delete-btn";
        // delBtn.textContent = "✕";
        // delBtn.title = "Delete";
        // delBtn.onclick = () => {
        //   if (confirm(`Delete "${file.name}"?`)) {
        //     deleteFile(file.name);
        //   }
        // };

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

// Initial Load
loadFiles();
