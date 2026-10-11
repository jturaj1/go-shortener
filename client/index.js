async function fetchWithAuth(url, options = {}) {
  const token = localStorage.getItem("token");
  const headers = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (response.status === 401) {
      localStorage.removeItem("token");
      updateUIBasedOnAuthentication();
      throw new Error("Unauthorized - Please login again");
    }

    return response;
  } catch (error) {
    console.error("Fetch error:", error);
    throw error;
  }
}

function isAuthenticated() {
  const token = localStorage.getItem("token");
  return token !== null && token !== "";
}

function updateUIBasedOnAuthentication() {
  const token = localStorage.getItem("token");
  const authRequiredDiv = document.getElementById("auth-required");
  const pleaseLoginDiv = document.getElementById("please-login");
  const logoutButton = document.getElementById("logout-btn");

  if (isAuthenticated()) {
    authRequiredDiv.style.display = "block";
    pleaseLoginDiv.style.display = "none";
    logoutButton.style.display = "block";
  } else {
    authRequiredDiv.style.display = "none";
    pleaseLoginDiv.style.display = "block";
    logoutButton.style.display = "none";
  }
}

document.addEventListener("DOMContentLoaded", function () {
  updateUIBasedOnAuthentication();

  if (isAuthenticated()) {
    getUrls().catch((error) => {
      console.error("Failed to load URLs", error);
    });
  } else {
    console.log("User is not authenticated");
  }
});

document
  .getElementById("urlForm")
  .addEventListener("submit", async function (event) {
    event.preventDefault();

    if (!isAuthenticated()) {
      window.location.href = "signin.html";
      return;
    }

    const form = event.target;
    const formData = new FormData(form);
    const url = formData.get("url");

    const resultDiv = document.getElementById("result");
    resultDiv.innerHTML = "";

    try {
      const response = await fetchWithAuth("http://localhost:8080/submit", {
        method: "POST",
        body: JSON.stringify({ url: url }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(
          `Failed to shorten URL: ${errorData.error || "Unknown error"}`,
        );
      }

      const data = await response.json();
      const shortUrl = data.received_url;

      resultDiv.innerHTML = `
      <p>Your URL: <a href="${shortUrl}" target="_blank">${shortUrl}</a></p>
      `;

      await getUrls();
    } catch (error) {
      console.error("Submit error:", error);
      resultDiv.innerHTML = `<p>Error: ${error.message}</p>`;
    }
  });

async function deleteUrl(shortCode) {
  try {
    const response = await fetchWithAuth(
      `http://localhost:8080/delete/${shortCode}`,
      {
        method: "DELETE",
      },
    );

    if (!response.ok) {
      const errorData = await response.json().carch(() => ({}));
      throw new Error(
        `Failed to delete URL: ${errorData.error || "Unknown error"}`,
      );
    }

    return await response.json();
  } catch (error) {
    console.error("Delete URL error", error);
    throw error;
  }
}

async function getUrls() {
  const url = "http://localhost:8080/urls";
  try {
    const response = await fetchWithAuth(url);
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(`HTTP Error: ${response.status}`);
    }

    const result = await response.json();
    console.log(result);

    const urlListDiv = document.getElementById("urlsList");
    if (result && result.urls && result.urls.length > 0) {
      urlListDiv.innerHTML = `
        <h2>Your URLs</h2>
        <ul>
          ${result.urls
            .map(
              (urlObj) => `
            <li>
            <a href="${urlObj.long_url}" target="_blank">${urlObj.long_url}</a>
            ->
            <a href="${urlObj.short_url}" target="_blank">${urlObj.short_url}</a>
            ${urlObj.click_count !== undefined ? ` (Clicks: ${urlObj.click_count})` : ""}
            <button class="delete-btn" data-shortcode="${urlObj.short_code}">Delete</button>
            </li>
          `,
            )
            .join("")}
        </ul>
      `;
      document.querySelectorAll(".delete-btn").forEach((button) => {
        button.addEventListener("click", async function () {
          const shortCode = this.getAttribute("data-shortcode");
          if (confirm("Are you sure you wanna delete this URL?")) {
            try {
              await deleteUrl(shortCode);
              await getUrls();
            } catch (error) {
              console.error("Delete error:", error);
              alert(`Error: ${error.message}`);
            }
          }
        });
      });
    } else {
      urlListDiv.innerHTML = `<p>No URLs found</p>`;
    }
  } catch (error) {
    console.error("Get URLs error:", error.message);
    const urlListDiv = document.getElementById("urlsList");
    urlListDiv.innerHTML = `<p>Error Fetching URLs: ${error.message}</p>`;
  }
}

document.getElementById("logout-btn").addEventListener("click", function (e) {
  e.preventDefault();
  localStorage.removeItem("token");
  updateUIBasedOnAuthentication();
});
