# v0.19 Blaze Music + Video shots, dark and light (called from ci/shots.py; same radius guard).
import pathlib


def run(b, root, out, mock, url, sample, guard):
    allerrs = []
    media = (root / "ci" / "media-mock.js").read_text()
    for theme in ("light", "dark"):
        pg = b.new_page(viewport={"width": 1280, "height": 820}, device_scale_factor=1)
        pg.add_init_script(f"window.__theme='{theme}';window.__sample='{sample}';" + mock + media)
        errs = []
        pg.on("pageerror", lambda e: errs.append(str(e)))
        guard(pg, errs)
        pg.goto(url); pg.wait_for_timeout(700)
        # Music: library (For you) with a song playing, Now playing on the right.
        pg.evaluate("show('music')"); pg.wait_for_timeout(500)
        pg.evaluate("""() => { const a = HTMLMediaElement.prototype; Object.defineProperty(a, 'src', { set(v) { this._s = v; }, get() { return this._s || ''; }, configurable: true }); a.play = function () { Object.defineProperty(this, 'paused', { value: false, configurable: true }); this.dispatchEvent(new Event('play')); return Promise.resolve(); };
          a.pause = function () { Object.defineProperty(this, 'paused', { value: true, configurable: true }); this.dispatchEvent(new Event('pause')); };
          BlazeMusic.playList(window.__media.songs, 0); }"""); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"music-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.mznp .mzctl') && document.querySelectorAll('.mzgrid .mztile').length >= 3"):
            errs.append("music: Now playing or mixes missing")
        pg.evaluate("document.querySelector('.mznp [data-p=lyrics]').click()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"music-lyrics-{theme}.png"))
        if "Light across the floor" not in pg.evaluate("document.querySelector('.mznp').innerText"):
            errs.append("music: lyrics missing")
        pg.evaluate("document.querySelector('.mzchip[data-s=playlists]').click()"); pg.wait_for_timeout(300)
        pg.evaluate("document.querySelector('.mznp [data-p=queue]').click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"music-playlists-queue-{theme}.png"))
        pg.evaluate("document.querySelector('.mzpill [data-t=store]').click()"); pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"music-store-ideas-{theme}.png"))
        pg.evaluate("document.querySelector('.mzchip.idea').click()"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"music-store-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.mzsgrid .mzitem').length") < 4:
            errs.append("music: Store results missing")
        pg.evaluate("document.querySelector('.mzsgrid .mzitem').click()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"music-store-item-{theme}.png"))
        pg.evaluate("document.querySelector('.mzsheet-bg').remove(); document.querySelector('.mzpill [data-t=downloads]').click()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"music-downloads-{theme}.png"))
        # The mini player stays on other tabs.
        pg.evaluate("show('work')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"music-mini-{theme}.png"))
        if not pg.evaluate("document.querySelector('#mzMini').classList.contains('in')"):
            errs.append("music: mini player should show outside Music")
        # Video: library, then the player with subtitles menu and recap.
        pg.evaluate("show('video')"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"video-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.mzvgrid .mzitem').length") < 5:
            errs.append("video: library tiles missing")
        pg.evaluate("BlazeVideo.play(window.__media.videos[1])"); pg.wait_for_timeout(700)
        pg.evaluate("document.querySelector('.mzv .rc').click()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"video-player-{theme}.png"))
        if "supplier" not in pg.evaluate("document.querySelector('.mzv-recap').innerText"):
            errs.append("video: recap missing")
        pg.evaluate("document.querySelector('.mzv .cc').click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"video-subs-{theme}.png"))
        if errs:
            print("JS errors:", errs); allerrs.extend(f"media {theme}: {e}" for e in errs)
        pg.close()
    return allerrs
