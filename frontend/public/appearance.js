try {
  const p = JSON.parse(
    localStorage.getItem("market-watch.preferences.v1") || "{}",
  );
  document.documentElement.dataset.theme =
    p.theme === "light" ? "light" : "dark";
} catch {}
