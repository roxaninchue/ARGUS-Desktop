# Screenshots of the UI with a fake backend, pushed to the `shots` branch by CI.
import pathlib, sys
from playwright.sync_api import sync_playwright
root = pathlib.Path(__file__).resolve().parent.parent
out = root / "shots"; out.mkdir(exist_ok=True)
mock = (root / "ci" / "mock.js").read_text()
url = (root / "frontend" / "dist" / "index.html").as_uri()
sample = (root / "ci" / "sample").as_uri()
rdash = (root / "internal" / "router" / "web" / "index.html").as_uri()
rstate = (root / "ci" / "router-state.json").read_text()
mock = f"window.__routerDash='{rdash}';window.__routerState={rstate};" + mock
allerrs = []
# v0.18.1: nothing square. Before every screenshot, every visible button, input, textarea, chip, pill,
# card, work card, step list, code block, composer, menu and popover must have a border radius.
# The allow-list is only parts drawn inside a rounded parent (rows of the step list, segmented buttons,
# code inside a code block) and the design stage's own content.
RADIUS_JS = r"""() => {
  const SEL = 'button, input:not([type=hidden]):not([type=range]):not([type=file]):not([type=color]), textarea, select, .chip, .pill, .tag, .fchip, .card, .acard, .work.live, .work.done, .steps, .step, pre, .code, .inbox, .pop, .menu, code, kbd, .kbd, .seg, .toggle input, .shavatar, #avatar';
  const ALLOW = '.steps .step, .seg button, .mseg button, pre code, .code code, .dstage *, .artboard *, iframe, .brframe, .tl *, .winctl button';
  const bad = [];
  document.querySelectorAll(SEL).forEach(e => {
    if (e.matches(ALLOW)) return;
    const r = e.getBoundingClientRect();
    if (r.width < 4 || r.height < 4 || r.bottom < 0 || r.top > innerHeight || r.right < 0 || r.left > innerWidth) return;
    const cs = getComputedStyle(e);
    if (cs.visibility === 'hidden' || cs.display === 'none' || +cs.opacity === 0) return;
    if (e.closest('[style*="display: none"]')) return;
    const z = ['borderTopLeftRadius','borderTopRightRadius','borderBottomLeftRadius','borderBottomRightRadius'].every(k => cs[k] === '0px');
    if (z) bad.push(e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + (typeof e.className === 'string' && e.className ? '.' + e.className.trim().split(/\s+/).join('.') : '') + ' < ' + (e.parentElement ? (e.parentElement.className || e.parentElement.tagName) : ''));
  });
  return [...new Set(bad)];
}"""
def guard(pg, errs):
    orig = pg.screenshot
    def shot(*a, **kw):
        bad = pg.evaluate(RADIUS_JS)
        if bad:
            errs.append("square corners before " + pathlib.Path(str(kw.get("path", ""))).name + ": " + ", ".join(bad[:8]))
        return orig(*a, **kw)
    pg.screenshot = shot
with sync_playwright() as p:
    b = p.chromium.launch(**({"executable_path": __import__("os").environ["CHROME"]} if __import__("os").environ.get("CHROME") else {}))
    # SHOTS_THEMES=light runs one theme (a quicker local run); CI takes both.
    for theme in (__import__("os").environ.get("SHOTS_THEMES") or "light,dark").split(","):
        pg = b.new_page(viewport={"width": 1280, "height": 820}, device_scale_factor=1)
        pg.add_init_script(f"window.__theme='{theme}';window.__sample='{sample}';" + mock)
        errs = []
        pg.on("pageerror", lambda e: errs.append(str(e)))
        guard(pg, errs)
        pg.goto(url); pg.wait_for_timeout(900)
        # v0.19: no sidebar. Home is the tab on launch (greeting, quick ask, the board in stacks).
        pg.screenshot(path=str(out / f"home-{theme}.png"))
        if pg.evaluate("view") != "home" or pg.evaluate("document.querySelectorAll('.hcard').length") < 6:
            errs.append("Home should open on launch with its cards")
        if pg.evaluate("!!document.querySelector('#side, #chats, #newChat')"):
            errs.append("the chats sidebar should be gone")
        if "Good afternoon, Luke." not in pg.evaluate("document.querySelector('.hgreet').textContent"):
            errs.append("Home greeting missing")
        if pg.evaluate("[...document.querySelectorAll('#pill button')].map(b => b.textContent).join('|')") != "Home|Chat|Projects":
            errs.append("tabs should be Home | Chat | Projects")
        pg.evaluate("document.querySelector('.hstack[data-key=\"s:finance\"] .hcard').click()"); pg.wait_for_timeout(300)
        pg.evaluate("document.querySelector('.hstack[data-key=\"s:finance\"]').scrollIntoView({block: 'center'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"home-stack-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.hstack[data-key=\"s:finance\"] .hcard').length") != 2:
            errs.append("tapping a stack should expand it")
        pg.evaluate("document.querySelector('.homepage').scrollTop = 0; document.querySelector('.hadd').click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"home-add-{theme}.png"))
        pg.evaluate("closePop(); document.querySelector('.hcard[data-id=w1] .hmenu').click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"home-menu-{theme}.png"))
        pg.evaluate("closePop()")
        # v0.19 one infinite chat: the tail first, older turns page in at the top, then the earlier chats.
        pg.evaluate("Shell.go('chat')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"onechat-{theme}.png"))
        if pg.evaluate("OneChat.state.start") <= 0 or not pg.evaluate("!!document.querySelector('.olderbar .olderbtn')"):
            errs.append("one chat should open on its last turns with older ones to load")
        pg.evaluate("""async () => { for (let i = 0; i < 6; i++) await OneChat.older(); document.querySelector('.archsep').scrollIntoView({block: 'start'}); }"""); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"onechat-earlier-{theme}.png"))
        if not pg.evaluate("OneChat.state.mainDone") or pg.evaluate("document.querySelectorAll('.archsep').length") < 2:
            errs.append("older messages and earlier chats should page into the timeline")
        pg.evaluate("show('history', 'postgres')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"history-search-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.hresults .r').length") < 2:
            errs.append("history search should find chat and earlier-chat messages")
        pg.evaluate("document.querySelector('.hresults .r').click()"); pg.wait_for_timeout(700)
        if pg.evaluate("view") != "work" or not pg.evaluate("!!document.querySelector('.msg.user.flash')"):
            errs.append("a search result should jump to the message")
        pg.evaluate("Shell.go('projects')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"projects-tab-{theme}.png"))
        if pg.evaluate("view") != "projects" or "HomeGlow" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("Projects tab should list projects")
        # The rest of the run works in the HomeGlow project, as before.
        pg.evaluate("window.__enterProject()"); pg.wait_for_timeout(400)
        pg.evaluate("loadConv(window.__conv); show('work')"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"project-chat-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.pcrumb')") or pg.evaluate("Shell.mode") != "projects":
            errs.append("a project chat should sit under Projects with a crumb back")
        pg.screenshot(path=str(out / f"chat-{theme}.png"))
        pg.evaluate("loadConv(window.__convAbout)"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"about-phone-{theme}.png"))
        if "Settings › Blaze Desktop" not in pg.evaluate("document.querySelector('#log').innerText"):
            errs.append("self-knowledge chat should show the pairing steps")
        pg.evaluate("loadConv(window.__conv)"); pg.wait_for_timeout(200)
        pg.evaluate("document.querySelector('#input').value='/'; onType()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"menu-{theme}.png"))
        pg.evaluate("closeMenu(); document.querySelector('#input').value='@hero'; onType()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"files-{theme}.png"))
        # v0.7.4: pickers narrow as you type. /skills then "sli" leaves slide-deck (Luke's bug report).
        pg.evaluate("""async () => { closeMenu(); const st = await go.main.App.State(); const sd = window.__skills.find(s => s.name === 'slide-deck');
          window.__sdWas = { state: sd.state, enabled: sd.enabled }; window.__sdAdded = !st.skills.some(s => s.startsWith('slide-deck:'));
          if (window.__sdAdded) st.skills.push('slide-deck: ' + sd.description); Object.assign(sd, { state: 'bundled', enabled: true });
          const i = document.querySelector('#input'); i.value = '/skills'; await onType(); await pickMenu(menu.idx);
          i.value = 'sli'; await onType(); }""")
        pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"skills-filter-{theme}.png"))
        if theme == "dark":
            pg.evaluate("async () => { const i = document.querySelector('#input'); i.value = 'xyz'; await onType(); }"); pg.wait_for_timeout(200)
            pg.screenshot(path=str(out / "skills-filter-empty.png"))
        pg.evaluate("""async () => { closeMenu(); document.querySelector('#input').value = ''; const st = await go.main.App.State();
          Object.assign(window.__skills.find(s => s.name === 'slide-deck'), window.__sdWas);
          if (window.__sdAdded) st.skills.splice(st.skills.findIndex(s => s.startsWith('slide-deck:')), 1); }""")
        pg.evaluate("closeMenu(); document.querySelector('#input').value=''; show('project','p1')"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"project-{theme}.png"))
        pg.evaluate("show('design')"); pg.wait_for_timeout(1200)
        pg.screenshot(path=str(out / f"design-{theme}.png"))
        pg.evaluate("""selectOn=true; drawStage(); window.postMessage({blaze:'selected', items:[{selector:'section.hero > h1', tag:'h1', text:'Warm light for every corner of home.', html:'<h1>Warm light</h1>', styles:{color:'rgb(27,27,27)', background:'rgba(0,0,0,0)', fontSize:'54px', padding:'0px', borderRadius:'0px'}}]}, '*')"""); pg.wait_for_timeout(500)
        pg.evaluate("document.querySelector('.sbacts [data-a=Restyle]').click()"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"design-select-{theme}.png"))
        # v0.8.0 Design studio: states board, critic, brand kit, phone, ship, directions, write-back toast.
        pg.evaluate("selection=null; selectOn=false; STUDIO.mode='states'; drawStage()"); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"design-states-{theme}.png"))
        pg.evaluate("STUDIO.mode=''; STUDIO.cards[3]={version:3,score:78,a11y:70,layout:75,speed:100,issues:['2 text elements with low contrast (worst 2.9:1 on p.muted, needs 4.5:1)','Scrolls sideways on phone (36px too wide)','1 image without alt text']}; D.head=3; drawStage(); drawVersions(); document.querySelector('#dscore').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"design-critic-{theme}.png"))
        pg.evaluate("closePop(); document.querySelector('#dbrand').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"design-brand-{theme}.png"))
        pg.evaluate("closePop(); document.querySelector('#dphone').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"design-phone-{theme}.png"))
        pg.evaluate("closePop(); document.querySelector('#dship').click()"); pg.wait_for_timeout(400)
        pg.evaluate("fire('ship', {stage:'build', line:'Collecting the site\\'s files…'}); fire('ship', {stage:'done', line:'Live at https://homeglow.pages.dev', url:'https://homeglow.pages.dev'})"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"design-ship-{theme}.png"))
        pg.evaluate("""closePop(); const u = window.__sample + '/index.html'; window.__dirs = [{handle:'direction-a',letter:'A',angle:'Bold and editorial',status:'done',url:u,path:'/tmp/a'},{handle:'direction-b',letter:'B',angle:'Warm and friendly',status:'done',url:u,path:'/tmp/b'},{handle:'direction-c',letter:'C',angle:'Minimal and precise',status:'running',now:'Editing index.html',url:u,path:'/tmp/c'}]; fire('directions', window.__dirs)"""); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"design-directions-{theme}.png"))
        pg.evaluate("window.__dirs=[]; fire('directions', []); document.querySelector('#dtl').click()"); pg.wait_for_timeout(400)
        pg.evaluate("window.postMessage({blaze:'edit', edit:{kind:'text', target:{tag:'h1'}, old_text:'Warm light', new_text:'Warm light for every corner'}}, '*')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"design-writeback-{theme}.png"))
        pg.evaluate("document.querySelector('#dtl').click(); document.querySelectorAll('#toasts .toast').forEach(t => t.remove())")
        pg.evaluate("show('work'); document.querySelector('#route').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"router-picker-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.pop .brpick').length") < 7:
            errs.append("BlazeRouter presets missing from the model popover")
        pg.evaluate("[...document.querySelectorAll('.pop .opt')].find(o => /Google Gemini/.test(o.textContent)).click()"); pg.wait_for_timeout(400)
        pg.evaluate("document.querySelector('.pop .models:not([style*=none])').scrollIntoView({block: 'center'})"); pg.wait_for_timeout(100)
        pg.screenshot(path=str(out / f"model-picker-{theme}.png"))
        # BlazeRouter: top-bar button, dashboard view (real dashboard page, stubbed /api), Settings card.
        pg.evaluate("closePop(); show('router')"); pg.wait_for_timeout(1200)
        pg.screenshot(path=str(out / f"router-home-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('#routerBtn .brdot.green') && !!document.querySelector('.brview iframe.brframe')"):
            errs.append("BlazeRouter button or dashboard view missing")
        pg.evaluate("show('router', 'providers')"); pg.wait_for_timeout(1200)
        pg.screenshot(path=str(out / f"router-providers-{theme}.png"))
        pg.evaluate("show('settings', 'models')"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"router-settings-{theme}.png"))
        pg.evaluate("document.querySelector('.brclassic a').click()"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"router-classic-{theme}.png"))
        pg.evaluate("show('settings', 'providers')"); pg.wait_for_timeout(600)
        if pg.evaluate("view") != "router":
            errs.append("old Settings › Providers link should open BlazeRouter")
        pg.evaluate("SetRouterOff = window.go.main.App.SetRouterOn(false).then(() => show('router'))"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"router-off-{theme}.png"))
        pg.evaluate("window.go.main.App.SetRouterOn(true).then(() => show('work'))"); pg.wait_for_timeout(400)
        # v0.11.0 shell: Chat | Projects, Search, avatar menu, /views, Exit, + menu, live card.
        pg.evaluate("show('work'); newChat()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"shell-chat-{theme}.png"))
        pg.evaluate("document.querySelector('#avatar').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"shell-avatar-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.pop.shmenu .opt').length") < 7:
            errs.append("avatar menu is missing items")
        pg.evaluate("closePop(); BlazeViews.picker()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"shell-views-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('#menu .mi').length") < 8:
            errs.append("/views picker is missing views")
        pg.evaluate("closeMenu(); document.querySelector('#plus').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"shell-plus-{theme}.png"))
        pg.evaluate("closeMenu(); setApproval('autopilot')"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"shell-modechip-{theme}.png"))
        pg.evaluate("setApproval('auto')"); pg.wait_for_timeout(200)
        pg.evaluate("Shell.search()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"shell-search-{theme}.png"))
        pg.evaluate("closeMenu(); fire('view:open', 'design')"); pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"shell-design-exit-{theme}.png"))
        if pg.evaluate("view") != "design" or pg.evaluate("getComputedStyle(document.querySelector('#exitChip')).display") == "none":
            errs.append("open_view design should open Design with an Exit chip")
        pg.evaluate("document.querySelector('#exitChip button').click()"); pg.wait_for_timeout(400)
        if pg.evaluate("view") != "work":
            errs.append("Exit should return to the chat")
        pg.evaluate("Shell.setMode('projects')"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"shell-projects-{theme}.png"))
        pg.evaluate("Shell.setMode('chat')"); pg.wait_for_timeout(400)
        if pg.evaluate("convId") != "main" or pg.evaluate("Shell.mode") != "chat":
            errs.append("the Chat tab should always open the one main conversation")
        pg.evaluate("window.__enterProject()"); pg.wait_for_timeout(400)
        pg.evaluate("""sendText('Make the hero section blue', []);
          for (let i = 0; i < 4; i++) { fire('tool', {id: 'lw'+i, name: 'file_read', args: '{"path":"index.html"}'}); fire('tool_done', {id: 'lw'+i, result: 'ok'}); }"""); pg.wait_for_timeout(1300)
        pg.screenshot(path=str(out / f"shell-livecard-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.work.live .wmeta')"):
            errs.append("live work card counter missing")
        # v0.13.0 long-chat engine: token meter in the live card, fresh-chat offer, project brief.
        pg.evaluate("fire('tokens', {in: 4200, out: 600, limit: 32768, high: false})"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"longchat-meter-{theme}.png"))
        if "in 4.2k" not in pg.evaluate("(document.querySelector('.work.live .wmeta') || {}).textContent || ''"):
            errs.append("token meter missing from the live card")
        pg.evaluate("fire('tokens', {in: 27500, out: 900, limit: 32768, high: true})"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"longchat-fresh-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.card.freshoffer .go')"):
            errs.append("fresh-chat offer missing")
        pg.evaluate("fire('done', {seconds: 72}); show('work')"); pg.wait_for_timeout(300)
        pg.evaluate("show('project', 'p1')"); pg.wait_for_timeout(700)
        pg.evaluate("document.querySelector('textarea.brief').scrollIntoView({block: 'center'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"longchat-brief-{theme}.png"))
        if not pg.evaluate("(document.querySelector('textarea.brief') || {}).value || ''").startswith("Purpose"):
            errs.append("project brief missing")
        pg.evaluate("show('work')"); pg.wait_for_timeout(200)
        # v0.16.0 one mind (v0.19: no sidebar): the one conversation, the read-only Archive, the idle-review toggle.
        pg.evaluate("OneChat.openMain()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"mind-onechat-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.olderbar')"):
            errs.append("one mind: the conversation should offer earlier messages")
        pg.evaluate("show('archive')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"mind-archive-{theme}.png"))
        if "Postgres" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("Archive view should list old chats")
        pg.evaluate("show('archive', 'c2')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"mind-archived-chat-{theme}.png"))
        if "homeglow-db" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("archived chat should be readable")
        pg.evaluate("show('settings', 'behaviour')"); pg.wait_for_timeout(600)
        pg.evaluate("(([...document.querySelectorAll('.group')].find(g => /Review past conversations/.test(g.textContent))) || document.body).scrollIntoView({block: 'center'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"mind-review-toggle-{theme}.png"))
        if "Review past conversations when idle" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("idle-review toggle missing")
        pg.evaluate("window.__enterProject()"); pg.wait_for_timeout(400)
        # v0.17.0 marketer pack: the seller skills in Settings › Skills, and the approval a WhatsApp send needs.
        mk = [x for x in __import__("json").loads((root / "skills" / "index.json").read_text())["skills"] if x["name"] in ("content-calendar", "product-listing", "ad-lab", "research-brief", "whatsapp-replies", "weekly-digest")]
        pg.evaluate("mk => { for (const x of mk) if (!window.__skills.some(s => s.name === x.name)) window.__skills.push({ ...x, state: 'bundled', enabled: true, update: false }); }", mk)
        pg.evaluate("show('settings','skills')"); pg.wait_for_timeout(600)
        pg.evaluate("document.querySelector('.sksearch').value='marketing'; document.querySelector('.sksearch').oninput()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"marketer-skills-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.skrows .skr').length") < 2:
            errs.append("marketer skills missing from Settings › Skills")
        pg.evaluate("document.querySelector('.sksearch').value=''; document.querySelector('.sksearch').oninput()"); pg.wait_for_timeout(200)
        pg.evaluate("""show('work'); loadConv(window.__conv);
          fire('confirm', {id: 'wa1', title: 'Send through your phone?', detail: 'Send WhatsApp to Ruth (HomeGlow order #1042): Hi Ruth, your Aurora lamp shipped today with DHL, tracking DX88213. It should reach Ndola by Friday. Thank you for shopping with HomeGlow!'});"""); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"marketer-whatsapp-approve-{theme}.png"))
        pg.evaluate("document.querySelectorAll('.card').forEach(c => { if (/Send through your phone/.test(c.textContent)) c.remove(); })")
        # v0.15.0 student pack: Learn view (flashcards) and low-data mode.
        pg.evaluate("show('learn')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"student-learn-{theme}.png"))
        if "NET103" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("Learn view should list decks")
        pg.evaluate("show('learn', 'NET103')"); pg.wait_for_timeout(400)
        pg.evaluate("[...document.querySelectorAll('#view button')].find(b => /Show answer/.test(b.textContent)).click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"student-card-{theme}.png"))
        if "/27" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("flashcard answer missing")
        pg.evaluate("show('settings', 'behaviour')"); pg.wait_for_timeout(600)
        pg.evaluate("(([...document.querySelectorAll('.group')].find(g => /Low-data mode/.test(g.textContent))) || document.body).scrollIntoView({block: 'center'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"student-lowdata-{theme}.png"))
        if "Low-data mode" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("low-data setting missing")
        pg.evaluate("show('work')"); pg.wait_for_timeout(200)
        # v0.14.0 Blaze Link: the Phone view (paired phone online, then the pairing QR code).
        pg.evaluate("show('phone')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"link-phone-{theme}.png"))
        if "Galaxy A12" not in pg.evaluate("document.querySelector('#view').innerText"):
            errs.append("Phone view should list the paired phone")
        pg.evaluate("[...document.querySelectorAll('#view button')].find(b => /Show pairing QR/.test(b.textContent)).click()"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"link-qr-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.lnk-qr')"):
            errs.append("pairing QR code missing")
        pg.evaluate("show('work')"); pg.wait_for_timeout(200)
        pg.evaluate("""closePop(); sendText('Make the hero section blue', []);
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          for (let i = 0; i < 8; i++) { fire('tool', {id: 'x'+i, name: i < 2 ? 'file_read' : 'file_edit', args: '{"path":"index.html"}'});
            fire('tool_done', {id: 'x'+i, result: i < 3 ? 'Read index.html (120 lines)' : "Error in file_edit: the arguments weren't valid JSON (unexpected end of JSON input). Call it again with a JSON object."}); }
          fire('repair', {dropped: ['a call with no tool name'], repairs: []});
          fire('error', {message: "Google Gemini refused the conversation's format (Request contains an invalid argument.). Blaze repaired the history; try again, or switch model.", detail: 'No model could answer:\\n- Google Gemini: HTTP 400: Request contains an invalid argument.', kind: 'model', failures: []});""")
        pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"error-{theme}.png"))
        # v0.4: hidden tool steps. Mid-turn shimmer status line, then the quiet "Worked for 12s".
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("""sendText('Find out why the build is slow and fix it', []);
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          fire('tool', {id: 'w1', name: 'file_read', args: '{"path":"vite.config.ts"}'});
          fire('tool_done', {id: 'w1', result: 'Read vite.config.ts (40 lines)'});
          fire('tool', {id: 'w2', name: 'web_search', args: '{"query":"vite slow build sourcemaps"}'});""")
        pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"working-{theme}.png"))
        pg.evaluate("""fire('tool_done', {id: 'w2', result: '5 results'});
          fire('tool', {id: 'w3', name: 'file_edit', args: '{"path":"vite.config.ts"}'});
          fire('tool_done', {id: 'w3', result: 'Edited vite.config.ts (1 replacement).'});
          fire('tool', {id: 'w4', name: 'run_shell', args: '{"command":"pnpm build"}'});
          fire('tool_done', {id: 'w4', result: 'exit 0 · built in 9.8s'});
          fire('delta', 'The build was making **full source maps** for production. I switched them to `hidden`, and `pnpm build` now takes **9.8s** instead of 41s.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 12.2});""")
        pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"worked-{theme}.png"))
        pg.evaluate("document.querySelector('.work.done').click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"worked-open-{theme}.png"))
        # v0.4.6: plain-words steps. Live: only plain words (the backend's raw "Running: …" status is
        # translated); done: one "Worked · 6 steps · 14s" line; expanded: each step's exact command and output.
        def steps_turn(pg, tag, size):
            pg.evaluate("""newChat().then(() => { sendText('Why is the build slow? Fix it and run the tests.', []);
              fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
              const T = window.__stepsTurn;
              T.slice(0, 3).forEach((t, i) => { fire('tool', {id: 'p' + i, name: t.name, args: JSON.stringify(t.args)}); fire('tool_done', {id: 'p' + i, result: t.result}); });
              fire('tool', {id: 'p3', name: T[3].name, args: JSON.stringify(T[3].args)}); fire('tool_done', {id: 'p3', result: T[3].result});
              fire('tool', {id: 'p4', name: T[4].name, args: JSON.stringify(T[4].args)});
              fire('status', 'Running: ' + T[4].args.command); })""")
            pg.wait_for_timeout(800)
            pg.screenshot(path=str(out / f"steps-{tag}live-{theme}.png"))
            live = pg.evaluate("document.querySelector('.work.live .wt').textContent")
            if live.strip() != "Running the tests" or any(c in live for c in "~;|$/"):
                errs.append("live step line is not plain words: " + repr(live))
            pg.evaluate("""const T = window.__stepsTurn;
              fire('tool_done', {id: 'p4', result: T[4].result});
              fire('tool', {id: 'p5', name: T[5].name, args: JSON.stringify(T[5].args)}); fire('tool_done', {id: 'p5', result: T[5].result});
              fire('delta', 'Production builds were writing **full source maps**. I set `build.sourcemap` to `hidden`; `pnpm build` now takes **9.8s** instead of 41s. One test still fails on float rounding in `price.ts`.');
              fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 14.2});""")
            pg.wait_for_timeout(500)
            pg.screenshot(path=str(out / f"steps-{tag}collapsed-{theme}.png"))
            line = pg.evaluate("[...document.querySelectorAll('.msg.blaze')].pop().querySelector('.work.done').textContent.trim()")
            if line != "Worked · 6 steps · 14s":
                errs.append("collapsed line: " + repr(line))
            pg.evaluate("""const m = [...document.querySelectorAll('.msg.blaze')].pop(); m.querySelector('.work.done').click();
              const rows = m.querySelectorAll('.step'); rows[0].querySelector('.head').click(); rows[4].querySelector('.head').click();""")
            pg.wait_for_timeout(400)
            pg.evaluate("[...document.querySelectorAll('.msg.blaze')].pop().querySelector('.work').scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
            pg.screenshot(path=str(out / f"steps-{tag}expanded-{theme}.png"))
            cmd = pg.evaluate("[...document.querySelectorAll('.msg.blaze')].pop().querySelector('.step .cmd').textContent")
            if not cmd.startswith("cat ~/Projects/blaze/package.json"):
                errs.append("expanded step lost its exact command: " + repr(cmd))
        steps_turn(pg, "", None)
        # v0.4.7: background agents. Dispatch, then keep chatting while the agent works.
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("""sendText('Fix the failing build and get the tests green. I will keep working on the hero copy.', []);
          fire('turn', {kind: 'user', conv: 'c7'});
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          fire('tool', {id: 'd1', name: 'dispatch_agent', args: '{"handle":"fix-build","task":"Fix the HomeGlow build"}'});
          fire('agent', {handle: 'fix-build', conv: 'c7', task: 'Fix the failing HomeGlow build (pnpm build), then run pnpm test and make sure all tests pass. Report what was wrong.', status: 'running', now: 'Starting', n: 0, started: Date.now() - 1000, seconds: 1});
          fire('tool_done', {id: 'd1', result: 'started: agent "fix-build" is working in the background.'});
          fire('delta', "On it. A background agent is fixing the build; I'll report back when it's green.");
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 1.4, handoff: true});
          window.__agentSteps.slice(0, 3).forEach(s => fire('agent_step', {handle: 'fix-build', step: s}));
          fire('agent_step', {handle: 'fix-build', step: {...window.__agentSteps[3], done: false, result: ''}});
          fire('agent', {handle: 'fix-build', conv: 'c7', status: 'running', now: 'Editing hero.liquid', n: 4, started: Date.now() - 42000, seconds: 42});
          sendText('While that runs: give me three punchier headlines for the hero.', []);
          fire('turn', {kind: 'user', conv: 'c7'});
          fire('delta', '1. **Light that feels like home.**\\n2. **Warm corners, every evening.**\\n3. **Glow, without the glare.**\\n\\nI like the first: short, and it says what you sell.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 2.1});""")
        pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"agent-running-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.agcard.running').length") != 1:
            errs.append("running agent card missing")
        if pg.evaluate("document.querySelector('.agcard .ag-now').textContent.trim()") != "Editing hero.liquid":
            errs.append("agent card does not show the plain step")
        # An approval from the agent shows on its card, not in the main chat.
        pg.evaluate("""fire('confirm', {id: 'k9', agent: 'fix-build', title: 'Run this command?', detail: 'pnpm add -D vite@6'});""")
        pg.wait_for_timeout(300)
        pg.evaluate("document.querySelector('.agcard').scrollIntoView({block: 'center'})"); pg.wait_for_timeout(150)
        pg.screenshot(path=str(out / f"agent-approval-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.agcard .ag-ask .ag-q')"):
            errs.append("agent approval did not land on its card")
        pg.evaluate("document.querySelector('.agcard .ag-q .yes').click()")
        # It finishes: the card collapses and its result wakes the main agent.
        pg.evaluate("""fire('agent', {handle: 'fix-build', conv: 'c7', status: 'done', now: '', n: 6, steps: window.__agentSteps, started: Date.now() - 95000, ended: Date.now(), seconds: 95,
            result: 'The build broke because `sections/hero.liquid` imported `./hero.css`, but the file is `assets/Hero.css` (capital H). Linux is case-sensitive, so the build failed there.\\n\\n- Fixed the import in `sections/hero.liquid`.\\n- `pnpm build` passes (9.8s).\\n- `pnpm test`: **42 passed**.'});
          fire('turn', {kind: 'agent', handle: 'fix-build', conv: 'c7'});
          fire('delta', 'The build is **green again**. The hero imported `./hero.css` but the file is `Hero.css`, and Linux cares about the capital. Fixed, and all **42 tests pass**.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 1.9});""")
        pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"agent-done-{theme}.png"))
        if pg.evaluate("document.querySelector('.agcard .ag-now').textContent.trim()") != "Done · 6 steps":
            errs.append("finished agent card did not collapse to Done · 6 steps")
        pg.evaluate("document.querySelector('.agcard .ag-now').click()"); pg.wait_for_timeout(300)
        pg.evaluate("document.querySelector('.agcard').scrollIntoView({block: 'start'})"); pg.wait_for_timeout(150)
        pg.screenshot(path=str(out / f"agent-expanded-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.agcard.open .ag-log .step').length") != 6:
            errs.append("expanded agent card does not list its 6 steps")
        # A saved chat after a restart: the agent shows as interrupted with Resume / Discard.
        pg.evaluate("loadConv(window.__agentConv)"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"agent-interrupted-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.agcard.interrupted .ag-acts .btn.accent')"):
            errs.append("interrupted agent card has no Resume button")
        # v0.6.0: orchestrator + specialists. The lead splits the task; a compact strip
        # above the composer shows each specialist (two run, one queues at the cap of 2 here).
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("""sendText('Add dark mode to HomeGlow: a toggle in the header, CSS variables for every colour, and contrast tests for both themes.', []);
          fire('turn', {kind: 'user', conv: 'c10'});
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          window.__specs.forEach((s, i) => {
            fire('tool', {id: 'sp' + i, name: 'dispatch_specialist', args: JSON.stringify({name: s.handle, brief: s.task})});
            fire('agent', {...s, status: i < 2 ? 'running' : 'queued', queue: i < 2 ? 0 : 1, now: 'Starting', n: 0, started: i < 2 ? Date.now() - (80 - i * 25) * 1000 : 0, seconds: i < 2 ? 80 - i * 25 : 0});
            fire('tool_done', {id: 'sp' + i, result: 'started: specialist "' + s.handle + '" (build).'});
          });
          fire('delta', "This splits cleanly, so I've briefed **three specialists**: the toggle, the colour variables and the contrast tests. Each works in its own worktree; I'll review their changes before anything touches your folder.");
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 2.3, handoff: true});
          window.__agentSteps.slice(0, 2).forEach(s => fire('agent_step', {handle: 'theme-toggle', step: s}));
          fire('agent_step', {handle: 'theme-toggle', step: {id: 'tt3', name: 'file_edit', args: '{"path":"sections/header.liquid"}', plain: 'Editing header.liquid', done: false, at: 'x'}});
          fire('agent_step', {handle: 'css-vars', step: {id: 'cv1', name: 'file_search', args: '{"pattern":"#[0-9a-f]{6}"}', plain: 'Searching your files', done: false, at: 'y'}});""")
        pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"orch-running-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('#specs .sprow').length") != 3:
            errs.append("specialists strip does not list 3 specialists")
        if pg.evaluate("document.querySelectorAll('#specs .sprow.live').length") != 2:
            errs.append("specialists strip does not show 2 running")
        if not pg.evaluate("!!document.querySelector('#specs .sprow.live .sp-mk .morb')"):
            errs.append("specialist rows have no mini orb")
        # Expand one: its plan, where it works, and its log so far.
        pg.evaluate("document.querySelector('#specs .sprow .sp-line').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"orch-expanded-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('#specs .sprow.open .sp-plan')"):
            errs.append("expanded specialist does not show its plan")
        pg.evaluate("document.querySelector('#specs .sprow.open .sp-line').click()")
        # css-vars finishes: its result wakes the lead, which reviews before applying.
        pg.evaluate("""fire('agent', {handle: 'css-vars', kind: 'specialist', conv: 'c10', status: 'done', review: 'pending', changes: window.__specChanges, now: '', n: 9,
            steps: window.__agentSteps.slice(0, 4), started: Date.now() - 140000, ended: Date.now(), seconds: 140,
            result: 'Moved **31 hard-coded colours** in 3 files to variables on `:root`, with a `[data-theme=dark]` set in the new `assets/theme-dark.css`.\\n\\n- `pnpm build` passes (9.4s).\\n- Left `header.liquid` alone as briefed.'});
          fire('agent', {handle: 'contrast-tests', kind: 'specialist', conv: 'c10', status: 'running', queue: 0, now: 'Starting', started: Date.now() - 2000, seconds: 2});
          fire('turn', {kind: 'agent', handle: 'css-vars', conv: 'c10'});
          fire('tool', {id: 'rv1', name: 'check_specialist', args: '{"name":"css-vars","diff":true}'});
          fire('tool_done', {id: 'rv1', result: '- css-vars · build · done'});
          fire('delta', '**css-vars** is in: 31 colours now use variables and the build passes. I checked the diff and it only touches the four CSS files, so it is ready to apply once the toggle lands. Two specialists are still working.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 3.1});""")
        pg.wait_for_timeout(300)
        pg.evaluate("document.querySelector('#specs .sprow.rv-pending .sp-line').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"orch-review-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('#specs .sprow.rv-pending .sp-acts .btn.accent')"):
            errs.append("finished specialist has no Apply button for review")
        if pg.evaluate("(document.querySelector('#specs .sp-sum') || {}).textContent") != "2 running · 1 to review":
            errs.append("specialists summary wrong: " + str(pg.evaluate("(document.querySelector('#specs .sp-sum') || {}).textContent")))
        pg.evaluate("""['theme-toggle', 'contrast-tests'].forEach(h => fire('agent', {handle: h, status: 'cancelled', ended: Date.now()}));
          fire('agent', {handle: 'css-vars', review: 'discarded'})""")
        # v0.4.2 AI chat titles: the sidebar rows they animated are gone in v0.19 (titles still show on the project page).
        # v0.4.2: mini particle-orb marks, zoomed 3x: a saved reply (frozen) and a live turn.
        zp = b.new_page(viewport={"width": 860, "height": 560}, device_scale_factor=3)
        zp.add_init_script(f"window.__theme='{theme}';window.__sample='{sample}';" + mock)
        zp.on("pageerror", lambda e: errs.append(str(e)))
        zp.goto(url); zp.wait_for_timeout(600)
        zp.evaluate("loadConv(window.__conv); show('work')"); zp.wait_for_timeout(300)
        zp.evaluate("document.querySelector('.work.done').click(); document.querySelector('#log').scrollTop = 0"); zp.wait_for_timeout(300)
        zp.locator(".msg.blaze").first.screenshot(path=str(out / f"marks-{theme}.png"))
        if zp.evaluate("document.querySelectorAll('.msg.blaze blaze-emblem.morb').length") < 3:
            errs.append("mini orb marks missing")
        zp.evaluate("""newChat().then(() => { sendText('Remember that my name is Luke', []);
          fire('tool', {id: 'm1', name: 'remember', args: '{"fact":"Name is Luke"}'}); fire('tool_done', {id: 'm1', result: 'Remembered.'});
          fire('tool', {id: 'm2', name: 'file_read', args: '{"path":"notes.md"}'}); document.querySelector('.work').click(); })""")
        zp.wait_for_timeout(700)
        zp.locator(".msg.blaze").first.screenshot(path=str(out / f"marks-live-{theme}.png"))
        zp.close()
        # v0.4.1: answer cards. A saved chat whose long table sits in a collapsed card above the short answer.
        pg.evaluate("loadConv(window.__cardConv); show('work')"); pg.wait_for_timeout(500)
        pg.evaluate("document.querySelector('#log').scrollTop = 0"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"card-collapsed-{theme}.png"))
        pg.evaluate("document.querySelector('.acard .atog').click()"); pg.wait_for_timeout(700)
        pg.evaluate("document.querySelector('.acard').scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"card-expanded-{theme}.png"))
        pg.evaluate("document.querySelector('#log').scrollTop = 1e6"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"card-expanded-end-{theme}.png"))
        # Streaming: an open :::card block fills its card live.
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("""sendText('Compare the roadmap phases', []);
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          fire('delta', 'Blaze Desktop is roughly **a third of the way to 1.0**.\\n\\n:::card Roadmap\\n' + window.__roadmap.split('\\n').slice(0, 5).join('\\n'));""")
        pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"card-streaming-{theme}.png"))
        pg.evaluate("""fire('delta', '\\n' + window.__roadmap.split('\\n').slice(5).join('\\n') + '\\n:::\\n');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 4.1});""")
        pg.wait_for_timeout(300)
        # Fallback: a long reply without card markers folds on "done".
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("""sendText('Set up a Postgres container for HomeGlow', []);
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          fire('delta', window.__autoReply); fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 6.3});""")
        pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"card-auto-{theme}.png"))
        n = pg.evaluate("document.querySelectorAll('.acard.tall').length + document.querySelectorAll('.acard.fits').length")
        if n < 1:
            errs.append("fallback reply did not fold into a card")
        # v0.5 composer: + menu, attachments (images, files, a folder), vision fallback, voice, modes.
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("openMenu('plus', plusItems(), 'Attach')"); pg.wait_for_timeout(250)
        pg.screenshot(path=str(out / f"composer-plus-{theme}.png"))
        pg.evaluate("""closeMenu(); addAttach(Object.values(window.__att));
          document.querySelector('#input').value = 'Why does the hero break on phones? The brief and the sections are attached.'; grow();""")
        pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"composer-attachments-{theme}.png"))
        n = pg.evaluate("document.querySelectorAll('#attach .achip').length")
        if n != 5:
            errs.append(f"expected 5 attachment chips, got {n}")
        pg.evaluate("document.querySelector('#attach .achip .ax').click()"); pg.wait_for_timeout(100)
        if pg.evaluate("document.querySelectorAll('#attach .achip').length") != 4:
            errs.append("removing a chip didn't work")
        pg.evaluate("S.vision_model = ''; drawAttach()"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"composer-novision-{theme}.png"))
        if not pg.evaluate("document.querySelector('#visionwarn').style.display !== 'none'"):
            errs.append("no vision warning without a vision model")
        pg.evaluate("S.vision_model = 'Google Gemini · gemini-3.8-flash'; submit()"); pg.wait_for_timeout(200)
        pg.evaluate("""fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          fire('delta', 'The hero image is a fixed **1440px** wide, so phones scroll sideways. I set it to `max-width: 100%` in `sections/hero.liquid`.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 5.2});""")
        pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"chat-attachments-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.msg.user .achip').length") != 4:
            errs.append("sent message lost its attachment chips")
        # Voice: the one-time setup card, its progress, then recording.
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("micClick()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"voice-setup-{theme}.png"))
        pg.evaluate("document.querySelector('#vcard .yes').click()"); pg.wait_for_timeout(100)
        pg.evaluate("fire('stt_progress', {stage: 'model', pct: 46, text: 'Downloading the Whisper base model… 65 of 142 MB'})"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"voice-progress-{theme}.png"))
        pg.evaluate("closeVoice(); recUI(true)"); pg.wait_for_timeout(1200)
        pg.screenshot(path=str(out / f"voice-recording-{theme}.png"))
        pg.evaluate("recUI(false); insertText('Make the hero image smaller on phones and keep the headline on one line.')"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"voice-text-{theme}.png"))
        # Modes: Plan (Ctrl+Tab) and Autopilot (Shift+Tab), from the keyboard.
        pg.evaluate("document.querySelector('#input').value=''; grow()")
        pg.focus("#input"); pg.keyboard.press("Control+Tab"); pg.wait_for_timeout(300)
        if pg.evaluate("MODES.work_mode") != "plan":
            errs.append("Ctrl+Tab didn't switch to Plan")
        pg.screenshot(path=str(out / f"mode-plan-{theme}.png"))
        pg.keyboard.press("Control+Tab"); pg.keyboard.press("Shift+Tab"); pg.wait_for_timeout(300)
        if pg.evaluate("MODES.work_mode + '/' + MODES.approval") != "build/autopilot":
            errs.append("Ctrl+Tab / Shift+Tab didn't give Build + Autopilot")
        pg.screenshot(path=str(out / f"mode-autopilot-{theme}.png"))
        pg.evaluate("""sendText('Upgrade the theme to the new Shopify CLI and make sure it still builds', []);
          fire('autopilot', {on: true, steps: 150, minutes: 45});
          fire('tool', {id: 'a1', name: 'run_shell', args: '{"command":"pnpm add -D @shopify/cli@latest"}'});
          fire('tool_done', {id: 'a1', result: 'exit 0'});
          fire('delta', 'The new CLI wants Node 20. Should I upgrade Node, or pin the old CLI?');
          fire('autoreply', 'The user is away');
          fire('delta', 'Pinned **@shopify/cli 3.66**, which still runs on your Node 18, and the theme builds. Assumption: upgrading Node is a bigger change than you asked for.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 48.0});""")
        pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"autopilot-reply-{theme}.png"))
        if not pg.evaluate("document.querySelector('.apnote')"):
            errs.append("no Autopilot note in the chat")
        pg.evaluate("setApproval('auto'); setWorkMode('build')"); pg.wait_for_timeout(200)
        # v0.4: the orb (quick ask), at its real 400x560 size.
        pg.set_viewport_size({"width": 400, "height": 560})
        pg.evaluate("newChat().then(() => fire('orb', true))"); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"orb-{theme}.png"))
        pg.evaluate("""sendText('How much disk space is left?', []);
          fire('tool', {id: 'o1', name: 'system_info', args: '{}'});""")
        pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"orb-working-{theme}.png"))
        pg.evaluate("""fire('tool_done', {id: 'o1', result: 'disk: 312 GB free of 1 TB'});
          fire('delta', '**312 GB free** of 1 TB. Your biggest folder is `~/code` (88 GB).');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 3.4});""")
        pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"orb-chat-{theme}.png"))
        steps_turn(pg, "orb-", None)
        pg.evaluate("""newChat().then(() => { sendText('How far is Blaze from 1.0?', []);
          fire('delta', window.__cardReply); fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 2.9}); })""")
        pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"card-orb-{theme}.png"))
        # v0.4.7: the orb's compact running-agents indicator, while the user asks something else.
        pg.evaluate("""newChat().then(() => { sendText('Research the three best budget laptops for coding and compare them', []);
          fire('turn', {kind: 'user', conv: 'c9'});
          fire('tool', {id: 'd2', name: 'dispatch_agent', args: '{}'});
          fire('agent', {handle: 'laptops', conv: 'c9', task: 'Research the three best budget laptops for coding (under 700 USD) and compare them in a table.', status: 'running', now: 'Reading notebookcheck.net', n: 5, started: Date.now() - 65000, seconds: 65});
          fire('agent', {handle: 'price-check', conv: 'c9', task: 'Check Zambian prices for the shortlisted laptops.', status: 'running', now: 'Searching the web for "ThinkPad E14 price Lusaka"', n: 2, started: Date.now() - 20000, seconds: 20});
          fire('tool_done', {id: 'd2', result: 'started: agent "laptops"'});
          fire('delta', "On it. Two agents are researching; I'll report back with a comparison."); fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 1.2, handoff: true}); })""")
        pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"orb-agents-{theme}.png"))
        if pg.evaluate("(document.querySelector('#orbbar .orbag') || {}).textContent") != "2 agents":
            errs.append("orb agents indicator missing")
        pg.evaluate("""['laptops', 'price-check'].forEach(h => fire('agent', {handle: h, status: 'cancelled', ended: Date.now()}))""")
        pg.evaluate("fire('orb', false)"); pg.set_viewport_size({"width": 1280, "height": 820}); pg.wait_for_timeout(300)
        # v0.4.8: the sun/moon theme switch in the top bar, flipped once and back.
        pg.evaluate("show('work')"); pg.wait_for_timeout(200)
        pg.evaluate("document.querySelector('#themeBtn').click()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"theme-flipped-{theme}.png"))
        flipped = pg.evaluate("document.documentElement.dataset.theme")
        if flipped == theme:
            errs.append("theme button didn't switch the theme")
        pg.evaluate("document.querySelector('#themeBtn').click()"); pg.wait_for_timeout(400)
        # v0.4.8: /update updates Blaze right in the chat (no Settings screen).
        pg.evaluate("newChat()"); pg.wait_for_timeout(300)
        pg.evaluate("""window.__upd = {current: 'v0.4.7', latest: 'v0.4.8', available: true, can_auto: true, method: 'Blaze downloads the setup and restarts.', page: ''};
          document.querySelector('#input').value = '/upd'; onType();""")
        pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"update-command-{theme}.png"))
        pg.evaluate("pickMenu(0)"); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"update-chat-{theme}.png"))
        if "42%" not in pg.evaluate("(document.querySelector('.selfupd .work') || {}).textContent || ''"):
            errs.append("/update progress line missing")
        if pg.evaluate("view") != "work":
            errs.append("/update left the chat")
        pg.evaluate("newChat(); window.__upd = {current: 'v0.4.8', latest: 'v0.4.8', available: false}"); pg.wait_for_timeout(400)
        pg.evaluate("SelfUpdate.run()"); pg.wait_for_timeout(500)
        if "newest version" not in pg.evaluate("(document.querySelector('.selfupd .md') || {}).textContent || ''"):
            errs.append("/update up-to-date answer missing")
        pg.screenshot(path=str(out / f"update-latest-{theme}.png"))
        pg.evaluate("window.__upd = null")
        # v0.5: Memory graph. Overview, the best-linked memory selected, mid-replay, sorting into categories.
        pg.evaluate("show('memory')"); pg.wait_for_timeout(700)
        pg.evaluate("MemoryUI.settle()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"memory-{theme}.png"))
        st = pg.evaluate("MemoryUI.stats()")
        if not st or st["nodes"] < 60 or st["edges"] < 30 or st["cats"] < 6:
            errs.append(f"memory graph is missing data: {st}")
        pg.evaluate("MemoryUI.select(MemoryUI.live.nodes.filter(n => n.kind === 'memory').sort((a, b) => b.deg - a.deg)[0].i)"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"memory-selected-{theme}.png"))
        if "In prompts" not in pg.evaluate("(document.querySelector('.mp-meta') || {}).textContent || ''"):
            errs.append("memory panel is missing the In prompts line")
        # v0.5.1: search uses the database's word-form search too ("deploying" finds "deploys").
        pg.evaluate("MemoryUI.select(-1)"); pg.wait_for_timeout(200)
        pg.fill(".msearch input", "deploying"); pg.wait_for_timeout(500)
        if not pg.evaluate("MemoryUI.live && MemoryUI.live.match && MemoryUI.live.match.size > 0"):
            errs.append("memory search found nothing for 'deploying'")
        pg.screenshot(path=str(out / f"memory-search-{theme}.png"))
        pg.evaluate("MemoryUI.live.input.value = ''; MemoryUI.live.search('')"); pg.wait_for_timeout(200)
        pg.evaluate("MemoryUI.seek(0.45); MemoryUI.play(4)"); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"memory-replay-{theme}.png"))
        pg.evaluate("MemoryUI.seek(0.93)"); pg.wait_for_timeout(1500)
        pg.screenshot(path=str(out / f"memory-sorting-{theme}.png"))
        pg.evaluate("show('work')"); pg.wait_for_timeout(200)
        if pg.evaluate("!!(MemoryUI.live && !MemoryUI.live.dead)"):
            errs.append("memory graph kept running after leaving the screen")
        for v in ["settings", "doctor", "schedule", "jobs"]:
            pg.evaluate(f"show('{v}')"); pg.wait_for_timeout(400)
            pg.screenshot(path=str(out / f"{v}-{theme}.png"))
        for s in ["providers", "usage", "behaviour", "accounts"]:
            pg.evaluate(f"show('settings','{s}')"); pg.wait_for_timeout(500)
            pg.screenshot(path=str(out / f"settings-{s}-{theme}.png"))
        # v0.4.2: Settings › Behaviour › Personality (Default, then Custom with the textarea).
        pg.evaluate("show('settings','behaviour')"); pg.wait_for_timeout(500)
        pg.evaluate("[...document.querySelectorAll('.group')].find(g => /Personality/.test(g.textContent)).scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"personality-{theme}.png"))
        pg.evaluate("[...document.querySelectorAll('.seg button')].find(b => b.textContent === 'Custom').click()"); pg.wait_for_timeout(500)
        pg.evaluate("[...document.querySelectorAll('.group')].find(g => /Personality/.test(g.textContent)).scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"personality-custom-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('textarea.pcustom')"):
            errs.append("custom personality textarea missing")
        # v0.4.x: Accounts (tabs All / Connections / Logins / API Keys) with real brand logos.
        pg.set_viewport_size({"width": 1280, "height": 1180})
        for t in ["all", "connections", "logins", "keys"]:
            pg.evaluate(f"show('accounts', '{t}')"); pg.wait_for_timeout(700)
            pg.screenshot(path=str(out / f"accounts-{t}-{theme}.png"))
        broken = pg.evaluate("[...document.querySelectorAll('.blogo img')].filter(i => !i.complete || !i.naturalWidth).map(i => i.src)")
        if broken:
            errs.append("brand logos failed to load: " + ", ".join(broken))
        pg.evaluate("document.querySelector('.aitem .arow').click()"); pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"accounts-keys-open-{theme}.png"))
        for v, arg in [("connector", "google"), ("connector", "spotify"), ("mcp", "linear")]:
            pg.evaluate(f"show('{v}', '{arg}')"); pg.wait_for_timeout(500)
            pg.screenshot(path=str(out / f"{v}-{arg}-{theme}.png"))
        # v0.5.0: one-click connectors through blaze-auth, and Blaze accounts.
        pg.set_viewport_size({"width": 1280, "height": 900})
        pg.evaluate("show('connector', 'slack')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"connector-oneclick-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.oneclick .btn.accent')"):
            errs.append("one-click Connect box missing on the Slack page")
        if pg.evaluate("!!document.querySelector('.csetupwrap[open]')"):
            errs.append("manual setup should be folded away when one-click is available")
        pg.evaluate("document.querySelector('.oneclick .btn.accent').click()"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"connector-oneclick-waiting-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.pend .waitline') && !document.querySelector('.pend details.paste')"):
            errs.append("one-click waiting box wrong (no wait line, or a paste box it doesn't need)")
        pg.evaluate("document.querySelector('.chead .acts .btn').click()"); pg.wait_for_timeout(400)
        pg.evaluate("show('accounts', 'all')"); pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"accounts-oneclick-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('.ucard.out')"):
            errs.append("signed-out account card missing on Accounts")
        pg.evaluate("show('signin')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"account-signin-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.signbox .obtn').length") != 2:
            errs.append("sign-in screen should offer Google and GitHub")
        pg.evaluate("document.querySelector('.signbox .sw').click()"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"account-signup-{theme}.png"))
        pg.fill(".signform input[name=email]", "kapapaluckson75@gmail.com"); pg.fill(".signform input[name=password]", "correct-horse-battery")
        pg.evaluate("document.querySelector('.signform .go').click()"); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"account-signed-in-{theme}.png"))
        if pg.evaluate("(document.querySelector('.ucard .un') || {}).textContent") != "Lackson Kapapa":
            errs.append("Settings › Account doesn't show the signed-in user")
        pg.evaluate("show('accounts', 'all')"); pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"accounts-signed-in-{theme}.png"))
        pg.evaluate("Object.assign(window.__acct, {signed_in: false, user: null}); show('settings', 'account')"); pg.wait_for_timeout(500)
        pg.evaluate("document.querySelector('.signbox .obtn').click()"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"account-signin-waiting-{theme}.png"))
        pg.evaluate("window.__acct.pending = ''; document.querySelector('#toasts') && document.querySelector('#toasts').remove()")
        pg.set_viewport_size({"width": 1280, "height": 820})
        pg.evaluate("""show('work'); loadConv(window.__conv);
          fire('connector_reauth', {id: 'github', name: 'GitHub'});
          fire('confirm', {id: 'k1', title: 'Send this email from Gmail?', detail: 'To: gloria@example.com\\nSubject: Fees plan', always: true});
          fire('connector_suggest', {id: 'slack', name: 'Slack', action: 'connect', reason: 'Blaze needs Slack to read #general for this.'});
          fire('toast', {text: 'Google is now connected', kind: 'ok'});"""); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"connector-cards-{theme}.png"))
        # v0.4.x: first-run onboarding, every step (plus macOS permissions).
        pg.evaluate("document.querySelector('#toasts') && document.querySelector('#toasts').remove()")
        for i, name in enumerate(["welcome", "model", "apps", "perms", "tour", "done"]):
            pg.evaluate(f"show('setup', {i})"); pg.wait_for_timeout(900)
            pg.screenshot(path=str(out / f"onboard-{i + 1:02d}-{name}-{theme}.png"))
        pg.evaluate("""window.go.main.App.State().then(s => { s.__d = s.desktop; s.desktop = { ...s.desktop, os: 'darwin', needs_access: true, hotkey: 'alt+space', hotkey_label: 'Option+Space', hotkey_ok: false, hotkey_note: 'macOS needs Accessibility permission for the shortcut.' }; return show('setup', 3); })"""); pg.wait_for_timeout(900)
        pg.screenshot(path=str(out / f"onboard-04-perms-mac-{theme}.png"))
        pg.evaluate("window.go.main.App.State().then(s => { s.desktop = s.__d; Onboard.close(); show('work'); })"); pg.wait_for_timeout(300)
        # notify-vision: Settings › Notifications (notifications, suggestions, screen vision, recent).
        pg.evaluate("show('settings','notifications')"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"settings-notifications-{theme}.png"))
        pg.evaluate("[...document.querySelectorAll('.group')].find(g => /Proactive suggestions/.test(g.textContent)).scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"settings-suggestions-{theme}.png"))
        pg.evaluate("[...document.querySelectorAll('.group')].find(g => /Screen vision/.test(g.textContent)).scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"settings-vision-{theme}.png"))
        if not pg.evaluate("/Proactive suggestions/.test(document.querySelector('#view').textContent)"):
            errs.append("Settings › Notifications did not render")
        # v0.8.1: Settings › MCP servers (shared with Blaze CLI via ~/.blaze/mcp.json).
        pg.evaluate("show('settings','mcp')"); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"settings-mcp-{theme}.png"))
        # The privacy indicator while the screen is read, then a clicked suggestion in the chat.
        pg.evaluate("loadConv(window.__conv); show('work'); fire('screen', {on: true, how: 'text'})"); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"screen-indicator-{theme}.png"))
        if not pg.evaluate("!!document.querySelector('#screenind.on')"):
            errs.append("privacy indicator did not show")
        pg.evaluate("fire('screen', {on: false})"); pg.wait_for_timeout(1800)
        if pg.evaluate("!!document.querySelector('#screenind.on')"):
            errs.append("privacy indicator did not hide")
        pg.evaluate("""fire('suggest', {id: 'n3', kind: 'suggest', title: 'pnpm test fails in price.ts', body: 'Expected 19.99, got 19.989999. I can fix the rounding.', prompt: 'Fix the failing price test'})"""); pg.wait_for_timeout(300)
        pg.screenshot(path=str(out / f"suggest-{theme}.png"))
        # v0.7: bundled skill library (Settings › Skills), Learning settings, and the "learned" marks in chat.
        pg.evaluate("show('settings','skills')"); pg.wait_for_timeout(600)
        pg.screenshot(path=str(out / f"skills-library-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.skrows .skr').length") < 15:
            errs.append("Settings › Skills lists fewer than 15 skills")
        pg.evaluate("document.querySelector('.sksearch').value='design'; document.querySelector('.sksearch').oninput(); [...document.querySelectorAll('.skacts .btn')].find(b => b.textContent === 'View').click()"); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"skills-filter-{theme}.png"))
        pg.evaluate("show('settings','behaviour')"); pg.wait_for_timeout(500)
        pg.evaluate("[...document.querySelectorAll('.group')].find(g => /Learn automatically/.test(g.textContent)).scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
        pg.screenshot(path=str(out / f"skills-learning-settings-{theme}.png"))
        if not pg.evaluate("/Learn automatically/.test(document.querySelector('#view').textContent)"):
            errs.append("Settings › Behaviour has no Learning group")
        pg.evaluate("""newChat().then(() => { sendText('From now on deploy HomeGlow with pnpm and the Shopify CLI - here is how: build, push the theme, then purge the CDN.', []);
          fire('route', {route: 'Google Gemini · gemini-3.8-flash'});
          fire('tool', {id: 'l1', name: 'remember', args: '{"fact":"Deploys HomeGlow with pnpm and the Shopify CLI"}'});
          fire('learned', {kind: 'memory', action: 'saved', text: 'Deploys HomeGlow with pnpm and the Shopify CLI', ref: '#41'});
          fire('tool_done', {id: 'l1', result: 'Remembered (#41).'});
          fire('tool', {id: 'l2', name: 'write_skill', args: '{"name":"deploy-homeglow"}'});
          fire('learned', {kind: 'skill', action: 'created', text: 'deploy-homeglow', ref: 'deploy-homeglow'});
          fire('tool_done', {id: 'l2', result: 'Saved skill deploy-homeglow.'});
          fire('delta', 'Got it. Next time I deploy HomeGlow I will run `pnpm build`, `shopify theme push` and purge the CDN.\\n\\nSaved this as the **deploy-homeglow** skill.');
          fire('done', {route: 'Google Gemini · gemini-3.8-flash', seconds: 3.1});
          fire('learned', {kind: 'memory', action: 'replaced', text: 'Prefers pnpm over npm in every project', ref: '#12', auto: true}); })"""); pg.wait_for_timeout(500)
        pg.screenshot(path=str(out / f"skills-learned-chat-{theme}.png"))
        if pg.evaluate("document.querySelectorAll('.learnmark').length") != 3:
            errs.append("learned marks missing in the chat")
        # v0.18.1 StreamOrb + shimmer mid-stream, and an agent run with the edge particles.
        st = pg.evaluate("window.__streamDemo()")
        pg.screenshot(path=str(out / f"chat-midstream-{theme}.png"))
        if not st or not st.get("pos") or st.get("glow", 0) < .5:
            errs.append("stream drop or shimmer missing mid-stream: " + str(st))
        if pg.evaluate("!!document.querySelector('.chat .t.typing') && getComputedStyle(document.querySelector('.chat .t.typing'), '::after').content !== 'none'"):
            errs.append("blinking caret still drawn")
        pg.evaluate("StreamOrb.thaw(); fire('done', {seconds: 2})"); pg.wait_for_timeout(1700)
        if pg.evaluate("!!document.querySelector('.sdrop, .sshim')") or not pg.evaluate("(([...document.querySelectorAll('#log .msg.blaze')].pop() || document).querySelector('.who blaze-emblem') || {hasAttribute: () => false}).hasAttribute('dim')"):
            errs.append("the drop should glide home and the pair rest dimmed beside the newest reply")
        if not pg.evaluate("window.__runDemo()"):
            errs.append("edge particles should be on while a run is active")
        pg.screenshot(path=str(out / f"agent-running-{theme}.png"))
        pg.evaluate("EdgeParticles.thaw(); fire('done', {seconds: 4})"); pg.wait_for_timeout(600)
        if pg.evaluate("getComputedStyle(document.querySelector('#edge-particles')).display") != "none":
            errs.append("edge particles should be gone when the run ends")
        # The indicator in the orb.
        pg.set_viewport_size({"width": 400, "height": 560})
        pg.evaluate("newChat().then(() => { fire('orb', true); fire('screen', {on: true, how: 'pixels'}); })"); pg.wait_for_timeout(700)
        pg.screenshot(path=str(out / f"orb-screen-{theme}.png"))
        pg.evaluate("fire('screen', {on: false}); fire('orb', false)"); pg.set_viewport_size({"width": 1280, "height": 820}); pg.wait_for_timeout(1700)
        if errs:
            print("JS errors:", errs); allerrs.extend(f"{theme}: {e}" for e in errs)
    # v0.18.1 Reduce motion (the Settings toggle): the drop just sits at the end, no shimmer, no particles,
    # no live emblems.
    pg = b.new_page(viewport={"width": 1280, "height": 820}, device_scale_factor=1)
    pg.add_init_script("localStorage.setItem('blaze.reduceMotion', '1');" + f"window.__theme='dark';window.__sample='{sample}';" + mock)
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)))
    guard(pg, errs)
    pg.goto(url); pg.wait_for_timeout(600)
    pg.evaluate("window.__streamDemo(300)")
    pg.screenshot(path=str(out / "chat-reduced-motion-dark.png"))
    still = pg.evaluate("""() => { const s = document.querySelector('.sshim'); const e = document.querySelector('#edge-particles');
      return { motion: BlazeMotion.enabled, shimmer: !!s && getComputedStyle(s).display !== 'none', edge: !!e && getComputedStyle(e).display !== 'none',
        drop: !!document.querySelector('.sdrop') }; }""")
    if still["motion"] or still["shimmer"] or still["edge"] or not still["drop"]:
        errs.append("reduced motion should leave only a still drop: " + str(still))
    if errs:
        print("JS errors:", errs); allerrs.extend("reduced-motion: " + e for e in errs)
    # Windows runs frameless with its own title-bar buttons.
    pg = b.new_page(viewport={"width": 1280, "height": 820}, device_scale_factor=1)
    pg.add_init_script(f"window.__theme='light';window.__os='windows';window.__sample='{sample}';" + mock)
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)))
    guard(pg, errs)
    pg.goto(url); pg.wait_for_timeout(600)
    pg.screenshot(path=str(out / "home-windows-light.png"))
    if errs:
        print("JS errors:", errs); allerrs.extend("windows: " + e for e in errs)
    # macOS: screen vision needs two permissions; Settings and Doctor offer to grant them.
    pg = b.new_page(viewport={"width": 1280, "height": 820}, device_scale_factor=1)
    pg.add_init_script(f"window.__theme='light';window.__os='darwin';window.__sample='{sample}';" + mock)
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)))
    guard(pg, errs)
    pg.goto(url); pg.wait_for_timeout(600)
    pg.evaluate("show('settings','notifications')"); pg.wait_for_timeout(600)
    pg.evaluate("[...document.querySelectorAll('.group')].find(g => /Screen vision/.test(g.textContent)).scrollIntoView({block: 'start'})"); pg.wait_for_timeout(200)
    pg.screenshot(path=str(out / "settings-vision-mac-light.png"))
    pg.evaluate("show('doctor')"); pg.wait_for_timeout(600)
    pg.evaluate("document.querySelector('#view .page').scrollTop = 1e6"); pg.wait_for_timeout(200)
    pg.screenshot(path=str(out / "doctor-mac-light.png"))
    if pg.evaluate("document.querySelectorAll('#view .fixbtn').length") < 2:
        errs.append("Doctor has no Fix buttons for the macOS permissions")
    if errs:
        print("JS errors:", errs); allerrs.extend("macos: " + e for e in errs)
    b.close()
# Shots still get pushed when something broke; ERRORS.txt says what (the job then fails).
if allerrs:
    (out / "ERRORS.txt").write_text("\n".join(allerrs) + "\n"); sys.exit(1)
print("ok")
