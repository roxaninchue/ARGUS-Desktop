# v0.17.1 mobile design: the shots Joey sees (home, messaging, composer states, rich replies,
# question card), light and dark. Run by ci/shots.py too (it imports run()).
import pathlib, sys, os
from playwright.sync_api import sync_playwright
root = pathlib.Path(__file__).resolve().parent.parent

RICH = r"""Here's the plan for the HomeGlow launch.

1. **Shoot the products** — daylight, white sweep, 3 angles each.
2. **Write the listings**: titles, bullets and SEO tags from the supplier pages.
3. **Build the store** — theme, collections and the shipping rules.
4. **Launch** with a 10% first-order code.

![Lamp](https://picsum.photos/seed/lamp/600/450)
The **Arc lamp** is the hero product: warm 2700K light, a linen shade and a brass stem. It photographs best at golden hour.

![a](https://picsum.photos/seed/a/600/450) ![b](https://picsum.photos/seed/b/600/450) ![c](https://picsum.photos/seed/c/600/450)

And the full set:

![a](https://picsum.photos/seed/c/600/450)
![b](https://picsum.photos/seed/d/600/450)
![c](https://picsum.photos/seed/e/600/450)
![d](https://picsum.photos/seed/a/600/450)

```mermaid
graph LR; Shoot-->List-->Store-->Launch
```

<<follow-ups: Write the listings | Draft launch posts | Price the lamps>>"""

PAL = {"lamp": ("#F6C27A", "#B86A2C"), "a": ("#9EC5FF", "#3D6FE0"), "b": ("#F3B3A6", "#C2513C"), "c": ("#B9E3C6", "#3D8A5A"), "d": ("#E8D5FF", "#7B4CC9"), "e": ("#FFE39A", "#C99A1E")}
def picture(url):
    key = next((k for k in PAL if f"/seed/{k}/" in url), "a"); a, b = PAL[key]
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 450"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="{a}"/><stop offset="1" stop-color="{b}"/></linearGradient></defs><rect width="600" height="450" fill="url(#g)"/><circle cx="300" cy="200" r="90" fill="#fff" opacity=".35"/><rect x="200" y="320" width="200" height="16" rx="8" fill="#fff" opacity=".5"/></svg>'
DIAGRAM = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 560 70" width="560" height="70" font-family="sans-serif" font-size="14">'
  + "".join(f'<rect x="{10+i*140}" y="15" width="110" height="40" rx="8" fill="#ECECFF" stroke="#9370DB"/><text x="{65+i*140}" y="40" text-anchor="middle">{t}</text>' + (f'<path d="M{120+i*140} 35h28" stroke="#333" marker-end="url(#m)"/>' if i < 3 else "") for i, t in enumerate(["Shoot", "List", "Store", "Launch"]))
  + '<defs><marker id="m" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto"><path d="M0 0 10 5 0 10z"/></marker></defs></svg>')

def run(out, themes=("light", "dark"), errs=None):
    errs = errs if errs is not None else []
    mock = (root / "ci" / "mock.js").read_text()
    url = (root / "frontend" / "dist" / "index.html").as_uri()
    sample = (root / "ci" / "sample").as_uri()
    rdash = (root / "internal" / "router" / "web" / "index.html").as_uri()
    rstate = (root / "ci" / "router-state.json").read_text()
    mock = f"window.__routerDash='{rdash}';window.__routerState={rstate};" + mock
    out.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as p:
        b = p.chromium.launch(**({"executable_path": os.environ["CHROME"]} if os.environ.get("CHROME") else {}))
        for theme in themes:
            pg = b.new_page(viewport={"width": 1280, "height": 820}, device_scale_factor=1)
            pg.add_init_script(f"window.__theme='{theme}';window.__sample='{sample}';" + mock)
            pe = []
            pg.on("pageerror", lambda e: pe.append(str(e)))
            # Offline in CI: pictures and mermaid.ink answer with local stand-ins.
            pg.route("**/picsum.photos/**", lambda r: r.fulfill(status=200, content_type="image/svg+xml", body=picture(r.request.url)))
            pg.route("**/mermaid.ink/**", lambda r: r.fulfill(status=200, content_type="image/svg+xml", body=DIAGRAM))
            pg.goto(url); pg.wait_for_timeout(700)
            pg.evaluate("show('work'); newChat()"); pg.wait_for_timeout(500)
            pg.screenshot(path=str(out / f"design-home-{theme}.png"))
            # messaging thread: a live turn that finishes with a step strip, then grouped replies
            pg.evaluate("""(async () => {
              await sendText('Why is the build slow? Fix it please', []);
              fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
              fire('tool', {id: 'w1', name: 'file_read', args: '{"path":"vite.config.ts"}'}); fire('tool_done', {id: 'w1', result: 'ok'});
              fire('tool', {id: 'w2', name: 'run_shell', args: '{"command":"pnpm build"}'}); fire('tool_done', {id: 'w2', result: 'exit 0'});
              fire('delta', 'The build was making **full source maps** for production. I switched them to `hidden`, and `pnpm build` now takes **9.8s** instead of 41s.');
              fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 12.2});
              const col = document.querySelector('#log .col');
              col.append(userMsg('Nice. Commit it')); col.append(userMsg('and push it to a new branch'));
              current = blazeMsg(); col.append(current.el); setBusy(true);
              fire('delta', 'Committed as `perf: hidden source maps` and pushed to **perf/sourcemaps**.\\n\\n<<follow-ups: Open a pull request | Run the tests | Show the diff>>');
              fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 3.1});
            })()"""); pg.wait_for_timeout(900)
            pg.screenshot(path=str(out / f"design-messaging-{theme}.png"))
            if pg.evaluate("document.querySelectorAll('.msg.user.g-first, .msg.user.g-last').length") < 2:
                errs.append("two quick user messages should group")
            if not pg.evaluate("!!document.querySelector('.msg.blaze.newest .fups .fup')"):
                errs.append("follow-up pills missing")
            if not pg.evaluate("!!document.querySelector('.msg.blaze .gm .ga[data-a=retry]')"):
                errs.append("group meta (Copy · Quote · Retry) missing")
            # composer: typing → send arrow; working → stop square
            pg.evaluate("document.querySelector('#input').value = 'Draft the release notes for v0.17.1'; document.querySelector('#input').dispatchEvent(new Event('input'))"); pg.wait_for_timeout(200)
            pg.locator("#composer").screenshot(path=str(out / f"design-composer-typing-{theme}.png"))
            pg.evaluate("document.querySelector('#input').value = ''; document.querySelector('#input').dispatchEvent(new Event('input'))"); pg.wait_for_timeout(200)
            pg.locator("#composer").screenshot(path=str(out / f"design-composer-idle-{theme}.png"))
            pg.evaluate("""sendText('Run the full test suite', []); fire('tool', {id: 'q1', name: 'run_shell', args: '{"command":"go test ./..."}'});"""); pg.wait_for_timeout(900)
            pg.screenshot(path=str(out / f"design-working-{theme}.png"))
            pg.evaluate("""fire('confirm', {id: 'c1', title: 'Run this command?', detail: 'git push --force origin perf/sourcemaps', always: true})"""); pg.wait_for_timeout(300)
            pg.screenshot(path=str(out / f"design-confirm-{theme}.png"))
            pg.evaluate("document.querySelector('.card.confirm .yes').click(); fire('done', {route: 'x', seconds: 1})"); pg.wait_for_timeout(200)
            # rich replies
            pg.evaluate("newChat()"); pg.wait_for_timeout(300)
            pg.evaluate("(async () => { await sendText('Plan the HomeGlow launch', []); fire('delta', %r); fire('done', {route: 'x', seconds: 2}); })()" % RICH); pg.wait_for_timeout(1500)
            pg.evaluate("document.querySelectorAll('.acard.tall:not(.open) .amore').forEach(b => b.click())"); pg.wait_for_timeout(600)
            pg.screenshot(path=str(out / f"design-rich-{theme}.png"))
            pg.evaluate("document.querySelector('#log').scrollTop = 99999"); pg.wait_for_timeout(300)
            pg.screenshot(path=str(out / f"design-rich-end-{theme}.png"))
            pg.evaluate("document.querySelector('.deck .dnav.next').click()"); pg.wait_for_timeout(600)
            bb = pg.locator(".md .deck").bounding_box()
            pg.screenshot(path=str(out / f"design-deck-{theme}.png"), clip={"x": bb["x"] - 40, "y": bb["y"] - 30, "width": bb["width"] + 80, "height": bb["height"] + 70})
            if pg.evaluate("document.querySelector('.deck').dataset.i") != "1":
                errs.append("deck arrows should flip to the next picture")
            # settings and views still rounded
            pg.evaluate("show('settings')"); pg.wait_for_timeout(500)
            pg.screenshot(path=str(out / f"design-settings-{theme}.png"))
            pg.evaluate("show('project','p1')"); pg.wait_for_timeout(500)
            pg.screenshot(path=str(out / f"design-project-{theme}.png"))
            pg.evaluate("show('design')"); pg.wait_for_timeout(1200)
            pg.screenshot(path=str(out / f"design-studio-{theme}.png"))
            if pe: errs.extend(f"{theme}: {e}" for e in pe)
            pg.close()
        b.close()
    return errs

if __name__ == "__main__":
    out = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else root / "shots"
    e = run(out, tuple((os.environ.get("SHOTS_THEMES") or "light,dark").split(",")))
    print("\n".join(e) or "design shots ok")
    sys.exit(1 if e else 0)
