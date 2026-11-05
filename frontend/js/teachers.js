(() => {
  const search = document.getElementById('teacherSearch');
  if (!search) return;

  const headSection = document.querySelector('.head-teacher');
  const grid = document.getElementById('teacherProfiles');
  const count = document.getElementById('teacherCount');
  const empty = document.getElementById('noTeachers');

  let profiles = [...document.querySelectorAll('.head-teacher, .teacher-card')];
  let searchTexts = [];

  function buildSearchIndex() {
    profiles = [...document.querySelectorAll('.head-teacher, .teacher-card')];
    searchTexts = profiles.map(profile => {
      const current = profile.textContent;
      const terms = Object.entries(typeof SCHOOL_BANGLA === 'undefined' ? {} : SCHOOL_BANGLA)
        .filter(([en, bn]) => current.includes(en) || current.includes(bn)).flat();
      return `${current} ${terms.join(' ')}`.toLocaleLowerCase();
    });
  }

  function update() {
    const query = search.value.trim().toLocaleLowerCase();
    let visible = 0;
    profiles.forEach((profile, index) => {
      profile.hidden = !(searchTexts[index] || '').includes(query);
      if (!profile.hidden) visible++;
    });
    count.textContent = document.documentElement.lang === 'bn'
      ? `${visible.toLocaleString('bn-BD')}টি নমুনা পরিচিতি দেখানো হচ্ছে`
      : `${visible} preview ${visible === 1 ? 'profile' : 'profiles'} displayed`;
    empty.hidden = visible !== 0;
  }

  function escapeHTML(str) {
    const d = document.createElement('div');
    d.textContent = str || '';
    return d.innerHTML;
  }

  function formatEmailParts(email) {
    const clean = String(email || '');
    const at = clean.indexOf('@');
    if (at === -1) return `<span>${escapeHTML(clean)}</span>`;
    return `<span>${escapeHTML(clean.slice(0, at))}</span><span>${escapeHTML(clean.slice(at))}</span>`;
  }

  function renderTeachers(list) {
    if (!Array.isArray(list) || !grid) return;
    const head = list.find(t => t.isHead) || list.find(t => String(t.role).toLowerCase() === 'head teacher');
    const staff = list.filter(t => t !== head);

    if (headSection) {
      if (head) {
        headSection.hidden = false;
        headSection.innerHTML = `
          <div class="teacher-portrait">
            <img src="${escapeHTML(head.image)}" alt="AI-generated fictional teacher portrait" width="800" height="1000" loading="lazy" />
          </div>
          <div class="head-copy">
            <span class="profile-role">School leadership</span>
            <h2 id="head-title">${escapeHTML(head.role || 'Head Teacher')}</h2>
            <p class="head-name">${escapeHTML(head.name)}</p>
            <dl>
              <div>
                <dt>Qualification</dt>
                <dd>${escapeHTML(head.qualification)}</dd>
              </div>
              <div class="profile-email">
                <dt>Email</dt>
                <dd><a href="mailto:${escapeHTML(head.email)}">${formatEmailParts(head.email)}</a></dd>
              </div>
            </dl>
            <a class="office-link" href="contact.html">Contact school office <span aria-hidden="true">↗</span></a>
          </div>`;
      } else {
        headSection.hidden = true;
      }
    }

    grid.innerHTML = staff.map(t => `
      <article class="teacher-card">
        <div class="teacher-portrait">
          <img src="${escapeHTML(t.image)}" alt="AI-generated fictional teacher portrait" width="800" height="1000" loading="lazy" />
        </div>
        <div class="profile-copy">
          <span class="profile-role">${escapeHTML(t.role)}</span>
          <h3>${escapeHTML(t.name)}</h3>
          <dl>
            <div>
              <dt>Subject</dt>
              <dd>${escapeHTML(t.subject)}</dd>
            </div>
            <div>
              <dt>Qualification</dt>
              <dd>${escapeHTML(t.qualification)}</dd>
            </div>
            <div class="profile-email">
              <dt>Email</dt>
              <dd><a href="mailto:${escapeHTML(t.email)}">${formatEmailParts(t.email)}</a></dd>
            </div>
          </dl>
        </div>
      </article>`).join('');

    buildSearchIndex();
    if (document.documentElement.lang === 'bn') {
      document.dispatchEvent(new CustomEvent('schoollanguagechange'));
    }
    update();
  }

  buildSearchIndex();
  search.addEventListener('input', update);
  document.addEventListener('schoollanguagechange', update);
  update();

  const apiURL = (location.protocol === 'file:' || (['localhost', '127.0.0.1', '[::1]'].includes(location.hostname) && location.port !== '8080'))
    ? 'http://localhost:8080/api/teachers'
    : '/api/teachers';

  fetch(apiURL, { cache: 'no-store' })
    .then(r => (r.ok ? r.json() : null))
    .then(data => {
      if (Array.isArray(data)) renderTeachers(data);
    })
    .catch(() => {});
})();
