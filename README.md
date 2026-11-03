# School Management System

A school website for S. M. Pilot Model High School, with a student portal.
Built as a two-person course project.

> Work in progress: pages are being added branch by branch.

## Project structure
```
frontend/    Website pages, styles, scripts, images and downloads
  css/       style.css, language.css, admission.css
  js/        script.js, language.js, admission.js
  images/    Logo and gallery photos
  downloads/ Admission policy, prospectus and application form
backend/     Go server for student login and the portal (coming soon)
```

## Run the website (frontend only)
```bash
cd frontend
python3 -m http.server 8080
```
Then open http://localhost:8080/home.html

## Pages currently in the repo
- Home
- Gallery
- Admission (overview, process, application form) and downloads

Language switching between English and Bangla is supported.

## Team and branches
| Branch | Account | Work |
|---|---|---|
| `Tusher` | AhmedTusher | Home, gallery, admission pages, backend |
| `Tahsin` | tahsin-rahman2004 | About, notice, achievements, contact pages |

Work is merged into `main` through Pull Requests.
