(() => {
  const reduceQuery = matchMedia('(prefers-reduced-motion: reduce)');
  const getPref = () => localStorage.getItem('blaze.reduceMotion') === '1';
  function syncMotion() {
    const forced = !!getPref();
    const enabled = !(reduceQuery.matches || forced);
    window.BlazeMotion = window.BlazeMotion || {};
    window.BlazeMotion.enabled = enabled;
    document.documentElement.classList.toggle('reduce-motion', !enabled);
    document.documentElement.classList.toggle('motion-off', !enabled);
    document.documentElement.style.setProperty('--motion-enabled', enabled ? '1' : '0');
  }
  reduceQuery.addEventListener?.('change', syncMotion);
  window.addEventListener('storage', e => { if (e.key === 'blaze.reduceMotion') syncMotion(); });
  syncMotion();
})();
