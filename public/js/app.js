function jakartaSeconds() {
  const parts = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Jakarta', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }).formatToParts();
  const value = (type) => Number(parts.find((part) => part.type === type)?.value || 0);
  return value('hour') * 3600 + value('minute') * 60 + value('second');
}

function tick() {
  const now = new Date();
  const clock = document.querySelector('#live-clock');
  if (clock) clock.textContent = now.toLocaleTimeString('en-GB');
  document.querySelectorAll('[data-countdown]').forEach((el) => {
    const [h, m] = el.dataset.countdown.split(':').map(Number);
    let diff = h * 3600 + m * 60 - jakartaSeconds();
    if (el.dataset.nextDay === 'true') diff += 24 * 3600;
    if (diff < 0) diff += 24 * 3600;
    el.textContent = `${String(Math.floor(diff / 60)).padStart(2, '0')}:${String(diff % 60).padStart(2, '0')}`;
  });
}
tick(); setInterval(tick, 1000);
if ('serviceWorker' in navigator) window.addEventListener('load', () => navigator.serviceWorker.register('/sw.js'));
