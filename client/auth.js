document.addEventListener("DOMContentLoaded", function () {
  document
    .getElementById("show-register")
    .addEventListener("click", function () {
      document.getElementById("login-form").style.display = "none";
      document.getElementById("register-form").style.display = "block";
    });

  document.getElementById("show-login").addEventListener("click", function () {
    document.getElementById("register-form").style.display = "none";
    document.getElementById("login-form").style.display = "block";
  });

  document
    .getElementById("loginForm")
    .addEventListener("submit", async function (e) {
      e.preventDefault();

      const email = document.getElementById("login-email").value;
      const password = document.getElementById("login-password").value;

      try {
        const response = await fetch("http://localhost:8080/login", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            email: email,
            password: password,
          }),
        });

        if (!response.ok) {
          const errorData = await response.json();
          throw new Error(errorData.error || "Login failed");
        }

        const data = await response.json();
        console.log(response.status);
        console.log(data);
        console.log(data.token);
        localStorage.setItem("token", data.token);

        window.location.href = "/static/index.html";
      } catch (error) {
        console.error("Login error", error);
        document.getElementById("login-error").textContent = error.message;
      }
    });

  document
    .getElementById("registerForm")
    .addEventListener("submit", async function (e) {
      e.preventDefault();

      const email = document.getElementById("register-email").value;
      const password = document.getElementById("register-password").value;
      const confirmPassword = document.getElementById(
        "register-confirm-password",
      ).value;
      const errorElement = document.getElementById("register-error");

      errorElement.textContent = "";

      if (password !== confirmPassword) {
        errorElement.textContent = "Passwords do not match";
        return;
      }

      try {
        const response = await fetch("http://localhost:8080/register", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            email: email,
            password: password,
          }),
        });

        if (!response.ok) {
          const errorData = await response.json();
          throw new Error(errorData.error || "Registration failed");
        }

        const loginResponse = await fetch("http://localhost:8080/login", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            email: email,
            password: password,
          }),
        });

        if (!loginResponse.ok) {
          const errorData = await loginResponse.json();
          throw new Error(errorData.error || "Auto-login failed");
        }

        const data = await loginResponse.json();
        localStorage.setItem("token", data.token);

        window.location.href = "index.html";
      } catch (error) {
        errorElement.textContent = error.message;
        console.error("Registration error:", error);
      }
    });

  const token = localStorage.getItem("token");
  if (token) {
    window.location.href = "/static/";
  }
});
