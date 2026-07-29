function tick() {
  const now = new Date();
  const clock = document.querySelector('#live-clock');
  if (clock) clock.textContent = now.toLocaleTimeString('en-GB');
  document.querySelectorAll('[data-countdown]').forEach((el) => {
    const [h, m] = el.dataset.countdown.split(':').map(Number);
    const departure = new Date(); departure.setHours(h, m, 0, 0);
    if (departure < now) departure.setDate(departure.getDate() + 1);
    const diff = Math.max(0, Math.floor((departure - now) / 1000));
    el.textContent = `${String(Math.floor(diff / 60)).padStart(2, '0')}:${String(diff % 60).padStart(2, '0')}`;
  });
}
tick(); setInterval(tick, 1000);
if ('serviceWorker' in navigator) window.addEventListener('load', () => navigator.serviceWorker.register('/sw.js'));
