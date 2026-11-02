// Generated from genesary/pupitre course-service/internal/db/seed/{courses,paths}/*.yaml
// (commit 3cdf7b2): the demo catalogue of the Go backend, used by the mock mode
// (docs/specs/006-mock-mode.md). Shapes follow definition.Course and definition.Path.


import type { CourseDefinition, PathDefinition } from '../model';

export const SEED_COURSES: CourseDefinition[] = [
 {
  "slug": "container-security",
  "spec": {
   "title": "Sécurité des conteneurs",
   "description": "Construire des images Docker sécurisées et déployer des workloads Kubernetes sans exposer inutilement la surface d'attaque.",
   "category": "security",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Gardien des conteneurs",
    "icon": "🛡️"
   },
   "modules": [
    {
     "name": "Surface d'attaque des conteneurs",
     "type": "text",
     "skills": [
      "containers",
      "cybersecurite"
     ],
     "content": "## Surface d'attaque des conteneurs\n\nLes conteneurs partagent le kernel de l'hôte. Une mauvaise configuration peut permettre à un attaquant de s'échapper du conteneur.\n\n### Le mode privileged — à éviter\n\n```bash\n# À NE JAMAIS faire en production\ndocker run --privileged monapp\n\n# Un conteneur privileged peut monter le filesystem de l'hôte :\nmount /dev/sda1 /mnt\nchroot /mnt\n# → accès complet au système hôte\n```\n\n### Linux Capabilities\n\nPar défaut, Docker supprime la plupart des capabilities Linux. Ne les réactive que si nécessaire.\n\n```bash\n# Mauvais : réactiver toutes les caps\ndocker run --cap-add=ALL monapp\n\n# Correct : ajouter uniquement celle dont tu as besoin\ndocker run --cap-add=NET_BIND_SERVICE monapp\n\n# Encore mieux : supprimer toutes les caps par défaut\ndocker run --cap-drop=ALL --cap-add=NET_BIND_SERVICE monapp\n```\n\n### Namespaces et isolation\n\n| Namespace | Isole |\n|---|---|\n| `pid` | Processus |\n| `net` | Interfaces réseau |\n| `mnt` | Système de fichiers |\n| `user` | UIDs/GIDs |\n| `ipc` | IPC (mémoire partagée) |\n\nLes conteneurs utilisent ces namespaces pour s'isoler du reste du système — mais pas du kernel.\n"
    },
    {
     "name": "Bonnes pratiques Dockerfile",
     "type": "text",
     "skills": [
      "containers",
      "cybersecurite"
     ],
     "content": "## Bonnes pratiques Dockerfile\n\n### Ne jamais tourner en root\n\n```dockerfile\n# Mauvais — tourne en root par défaut\nFROM node:22\nCOPY . .\nCMD [\"node\", \"server.js\"]\n\n# Correct — utilisateur non-privilégié\nFROM node:22-alpine\nWORKDIR /app\nCOPY --chown=node:node . .\nUSER node\nCMD [\"node\", \"server.js\"]\n```\n\n### Multi-stage build — ne pas inclure les outils de build\n\n```dockerfile\n# Stage 1 : compilation (outils de build présents)\nFROM golang:1.23-alpine AS builder\nWORKDIR /app\nCOPY . .\nRUN go build -o /bin/server .\n\n# Stage 2 : image finale sans SDK Go\nFROM gcr.io/distroless/static-debian12\nCOPY --from=builder /bin/server /server\nENTRYPOINT [\"/server\"]\n```\n\nL'image finale ne contient que le binaire : pas de shell, pas de package manager, surface d'attaque minimale.\n\n### Images minimales\n\n| Image | Taille | Shell | Package manager |\n|---|---|---|---|\n| `ubuntu:latest` | ~77 MB | ✅ | ✅ |\n| `alpine:3.20` | ~7 MB | ✅ (ash) | ✅ (apk) |\n| `distroless/static` | ~2 MB | ❌ | ❌ |\n| `scratch` | 0 MB | ❌ | ❌ |\n\n### Scanner les vulnérabilités\n\n```bash\n# Trivy — scan de CVEs dans une image\ntrivy image monapp:latest\n\n# Grype — alternative\ngrype monapp:latest\n\n# Dans GitLab CI\ncontainer_scanning:\n  stage: test\n  image: registry.gitlab.com/security-products/container-scanning:7\n```\n"
    },
    {
     "name": "Sécurité Kubernetes",
     "type": "text",
     "skills": [
      "containers",
      "cybersecurite"
     ],
     "content": "## Sécurité Kubernetes\n\n### RBAC — Role-Based Access Control\n\nPrincipe du moindre privilège : chaque service account ne doit avoir accès qu'à ce dont il a besoin.\n\n```yaml\n# Role — permissions dans un namespace\napiVersion: rbac.authorization.k8s.io/v1\nkind: Role\nmetadata:\n  namespace: production\n  name: pod-reader\nrules:\n  - apiGroups: [\"\"]\n    resources: [\"pods\"]\n    verbs: [\"get\", \"list\"]  # pas de create/delete/patch\n---\napiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata:\n  name: read-pods\n  namespace: production\nsubjects:\n  - kind: ServiceAccount\n    name: monitoring-sa\nroleRef:\n  kind: Role\n  name: pod-reader\n  apiGroup: rbac.authorization.k8s.io\n```\n\n### Pod Security Standards\n\n```yaml\n# Restreindre les pods d'un namespace\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: production\n  labels:\n    pod-security.kubernetes.io/enforce: restricted\n    # restricted = pas de root, pas de privileged, caps limités\n```\n\n### Network Policies\n\nPar défaut, tous les pods peuvent communiquer entre eux. Les NetworkPolicies limitent ce trafic.\n\n```yaml\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: deny-all-ingress\n  namespace: production\nspec:\n  podSelector: {}   # tous les pods\n  policyTypes:\n    - Ingress\n  # pas de règle ingress = tout bloquer\n```\n\n### Secrets Kubernetes — chiffrement at-rest\n\n```yaml\n# EncryptionConfiguration — à activer sur kube-apiserver\napiVersion: apiserver.config.k8s.io/v1\nkind: EncryptionConfiguration\nresources:\n  - resources:\n      - secrets\n    providers:\n      - aescbc:\n          keys:\n            - name: key1\n              secret: <base64-encoded-32-byte-key>\n      - identity: {}\n```\n"
    },
    {
     "name": "Quiz — Sécurité des conteneurs",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "surface-d-attaque-des-conteneurs",
      "bonnes-pratiques-dockerfile",
      "sécurité-kubernetes"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Pourquoi éviter l'option --privileged sur un conteneur Docker ?",
       "answers": [
        {
         "id": "a",
         "text": "Elle donne accès complet au kernel et au filesystem de l'hôte",
         "correct": true
        },
        {
         "id": "b",
         "text": "Elle consomme trop de mémoire",
         "correct": false
        },
        {
         "id": "c",
         "text": "Elle désactive le réseau du conteneur",
         "correct": false
        },
        {
         "id": "d",
         "text": "Elle empêche l'utilisation de volumes",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quel avantage principal offre un multi-stage Dockerfile ?",
       "answers": [
        {
         "id": "a",
         "text": "L'image finale ne contient pas les outils de build, réduisant la surface d'attaque",
         "correct": true
        },
        {
         "id": "b",
         "text": "Le build est plus rapide grâce au parallélisme",
         "correct": false
        },
        {
         "id": "c",
         "text": "Il permet de faire tourner plusieurs apps dans le même conteneur",
         "correct": false
        },
        {
         "id": "d",
         "text": "Il compresse automatiquement les layers",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Qu'est-ce qu'une NetworkPolicy Kubernetes avec policyType Ingress mais sans règle ?",
       "answers": [
        {
         "id": "a",
         "text": "Elle bloque tout le trafic entrant vers les pods ciblés",
         "correct": true
        },
        {
         "id": "b",
         "text": "Elle autorise tout le trafic entrant",
         "correct": false
        },
        {
         "id": "c",
         "text": "Elle bloque aussi le trafic sortant",
         "correct": false
        },
        {
         "id": "d",
         "text": "Elle n'a aucun effet",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "cybersecurity-intro",
  "spec": {
   "title": "Introduction à la cybersécurité",
   "description": "Découvre les bases de la cybersécurité : OWASP Top 10, gestion des identités, chiffrement et bonnes pratiques de sécurité applicative. Indispensable pour tout développeur.",
   "category": "security",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Sentinelle du Web",
    "icon": "🛡️"
   },
   "modules": [
    {
     "name": "Les menaces du web",
     "type": "text",
     "skills": [
      "cybersecurite"
     ],
     "content": "## Les menaces du web\n\nLes applications web sont exposées à de nombreuses attaques. Connaître les vecteurs les plus courants est la première étape pour s'en protéger.\n\n| Menace | Description |\n|---|---|\n| **Injection SQL** | Injection de code SQL dans un champ de formulaire pour manipuler la base de données |\n| **XSS** | Injection de scripts malveillants dans des pages vues par d'autres utilisateurs |\n| **CSRF** | Forcer un utilisateur authentifié à exécuter une action non désirée |\n| **IDOR** | Accès à des ressources d'autres utilisateurs en modifiant un identifiant |\n| **Broken Auth** | Mécanismes d'authentification mal implémentés |\n\n```http\n# Exemple d'injection SQL\nPOST /login HTTP/1.1\nusername=admin'--&password=anything\n\n# La requête SQL générée :\nSELECT * FROM users WHERE username='admin'--' AND password='anything'\n-- Le -- commente le reste, le mot de passe est ignoré\n```\n\n### Principe de défense en profondeur\n\n- Ne jamais faire confiance aux données en entrée\n- Valider côté serveur (jamais uniquement côté client)\n- Appliquer le principe du moindre privilège\n- Logger et monitorer les accès suspects\n"
    },
    {
     "name": "OWASP Top 10",
     "type": "text",
     "skills": [
      "cybersecurite"
     ],
     "content": "## OWASP Top 10\n\nL'OWASP (Open Web Application Security Project) publie régulièrement les 10 risques les plus critiques pour la sécurité des applications web.\n\n### Top 10 (2021)\n\n| Rang | Risque |\n|---|---|\n| A01 | Broken Access Control |\n| A02 | Cryptographic Failures |\n| A03 | Injection |\n| A04 | Insecure Design |\n| A05 | Security Misconfiguration |\n| A06 | Vulnerable & Outdated Components |\n| A07 | Identification & Authentication Failures |\n| A08 | Software & Data Integrity Failures |\n| A09 | Security Logging & Monitoring Failures |\n| A10 | Server-Side Request Forgery (SSRF) |\n\n### Exemples concrets\n\n**A01 - Broken Access Control**\n```http\nGET /api/users/42/documents   # ton compte\nGET /api/users/43/documents   # compte d'un autre → doit retourner 403\n```\n\n**A03 - Injection**\n```python\n# Vulnérable\nquery = f\"SELECT * FROM users WHERE id = {user_input}\"\n\n# Sécurisé (requête paramétrée)\nquery = \"SELECT * FROM users WHERE id = $1\"\ncursor.execute(query, (user_input,))\n```\n"
    },
    {
     "name": "Authentification et gestion des sessions",
     "type": "text",
     "skills": [
      "cybersecurite"
     ],
     "content": "## Authentification et gestion des sessions\n\nL'authentification vérifie qui tu es. La gestion des sessions maintient cette identité entre les requêtes.\n\n### Bonnes pratiques pour les mots de passe\n\n```python\nimport bcrypt\n\n# Hachage (à l'inscription)\npassword = b\"mon_mot_de_passe\"\nhashed = bcrypt.hashpw(password, bcrypt.gensalt(rounds=12))\n\n# Vérification (à la connexion)\nbcrypt.checkpw(password, hashed)  # True/False\n```\n\n> Ne jamais stocker un mot de passe en clair ou en MD5/SHA1 — utiliser bcrypt, argon2 ou scrypt.\n\n### JWT (JSON Web Token)\n\n```\nHeader.Payload.Signature\n\neyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyMSIsInJvbGUiOiJzdHVkZW50IiwiZXhwIjoxNzAwMDAwMDAwfQ.abc123\n```\n\n- **Header** : algorithme de signature\n- **Payload** : claims (sub, exp, role…) — lisible par tous, ne pas y mettre de données sensibles\n- **Signature** : garantit l'intégrité, vérifiée par le serveur avec la clé secrète\n\n### Sécurisation des sessions\n\n```http\nSet-Cookie: session=xyz; HttpOnly; Secure; SameSite=Strict; Max-Age=3600\n```\n\n- `HttpOnly` : interdit l'accès au cookie via JavaScript (protection XSS)\n- `Secure` : cookie transmis uniquement en HTTPS\n- `SameSite=Strict` : protection CSRF\n"
    },
    {
     "name": "Chiffrement et cryptographie",
     "type": "text",
     "skills": [
      "cybersecurite"
     ],
     "content": "## Chiffrement et cryptographie\n\nLa cryptographie protège les données en transit et au repos.\n\n### Chiffrement symétrique vs asymétrique\n\n| | Symétrique | Asymétrique |\n|---|---|---|\n| **Clés** | Une seule clé partagée | Paire publique/privée |\n| **Vitesse** | Rapide | Lent |\n| **Algorithme** | AES-256 | RSA, Ed25519 |\n| **Usage** | Données au repos | Échange de clés, signatures |\n\n### TLS (HTTPS)\n\n```\n1. Client → Serveur : ClientHello (versions TLS supportées)\n2. Serveur → Client : Certificat (clé publique + identité)\n3. Échange de clé de session (chiffrement asymétrique)\n4. Communication chiffrée (AES avec la clé de session)\n```\n\n### Bonnes pratiques\n\n```bash\n# Générer une clé AES-256\nopenssl rand -hex 32\n\n# Chiffrer un fichier\nopenssl enc -aes-256-cbc -salt -in secret.txt -out secret.enc\n\n# Générer une paire de clés Ed25519\nssh-keygen -t ed25519 -C \"mon@email.com\"\n```\n\n- Toujours utiliser TLS 1.2+ pour les communications réseau\n- Ne jamais implémenter sa propre cryptographie\n- Renouveler les certificats avant expiration (Let's Encrypt automatise ça)\n"
    },
    {
     "name": "Quiz Cybersécurité",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "les-menaces-du-web",
      "owasp-top-10",
      "authentification-et-gestion-des-sessions",
      "chiffrement-et-cryptographie"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle attaque consiste à injecter du SQL dans un formulaire ?",
       "answers": [
        {
         "id": "a",
         "text": "SQL Injection",
         "correct": true
        },
        {
         "id": "b",
         "text": "XSS",
         "correct": false
        },
        {
         "id": "c",
         "text": "CSRF",
         "correct": false
        },
        {
         "id": "d",
         "text": "Buffer Overflow",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Que signifie HTTPS ?",
       "answers": [
        {
         "id": "a",
         "text": "HyperText Transfer Protocol Secure",
         "correct": true
        },
        {
         "id": "b",
         "text": "High Transfer Protocol Standard",
         "correct": false
        },
        {
         "id": "c",
         "text": "HyperText Transport Protocol Safe",
         "correct": false
        },
        {
         "id": "d",
         "text": "Hosted Transfer Protocol Security",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quelle méthode de hachage est recommandée pour les mots de passe ?",
       "answers": [
        {
         "id": "a",
         "text": "bcrypt",
         "correct": true
        },
        {
         "id": "b",
         "text": "MD5",
         "correct": false
        },
        {
         "id": "c",
         "text": "SHA-1",
         "correct": false
        },
        {
         "id": "d",
         "text": "Base64",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "devsecops",
  "spec": {
   "title": "DevSecOps — Sécurité dans le pipeline",
   "description": "Intégrer la sécurité dès le début du cycle de développement — SAST, analyse de dépendances, scan d'images et supply chain security dans GitLab CI/CD.",
   "category": "security",
   "difficulty": "advanced",
   "public": true,
   "badge": {
    "name": "Expert DevSecOps",
    "icon": "🔐"
   },
   "modules": [
    {
     "name": "Shift-left security",
     "type": "text",
     "skills": [
      "devsecops",
      "securite"
     ],
     "content": "## Shift-left security\n\nLe \"shift-left\" consiste à déplacer les contrôles de sécurité le plus tôt possible dans le cycle de développement — là où les corrections coûtent le moins cher.\n\n### Coût d'un bug selon le moment de détection\n\n```\nDéveloppement  → correction : 1x\nCode review    → correction : 6x\nQA / tests     → correction : 15x\nProduction     → correction : 100x\n```\n\n### Les catégories d'outils DevSecOps\n\n| Type | Acronyme | Ce qu'il fait |\n|---|---|---|\n| Static Analysis | **SAST** | Analyse le code source sans l'exécuter |\n| Dynamic Analysis | **DAST** | Teste l'app en cours d'exécution |\n| Dependency Check | **SCA** | Détecte les librairies avec CVEs |\n| Container Scan | — | Cherche des CVEs dans les images Docker |\n| Secret Detection | — | Cherche des secrets dans le code |\n| IaC Scan | — | Vérifie la sécurité des fichiers Terraform/K8s |\n\n### Principe : security as code\n\nChaque contrôle est du code versionné, revu et automatisé — pas une checklist manuelle.\n\n```yaml\n# .gitlab-ci.yml — pipeline qui inclut tous les scans\ninclude:\n  - template: Security/SAST.gitlab-ci.yml\n  - template: Security/Dependency-Scanning.gitlab-ci.yml\n  - template: Security/Container-Scanning.gitlab-ci.yml\n  - template: Security/Secret-Detection.gitlab-ci.yml\n```\n\nEn un seul `include`, GitLab ajoute quatre types de scans automatiques à chaque MR.\n"
    },
    {
     "name": "SAST et analyse de dépendances",
     "type": "text",
     "skills": [
      "devsecops",
      "securite"
     ],
     "content": "## SAST et analyse de dépendances\n\n### SAST — Static Application Security Testing\n\n```yaml\n# GitLab — activation du SAST\ninclude:\n  - template: Security/SAST.gitlab-ci.yml\n\nvariables:\n  SAST_EXCLUDED_PATHS: \"vendor, node_modules, dist\"\n```\n\nGitLab utilise différents analyseurs selon le langage détecté :\n- **Go** → gosec\n- **Python** → bandit\n- **JavaScript** → eslint-security\n- **Java** → SpotBugs\n\nExemple de résultat gosec :\n\n```\n[MEDIUM] G401: Use of weak cryptographic primitive (MD5)\n  File: internal/auth/hash.go:12\n  Code: h := md5.New()\n\n[HIGH] G501: Import blocklist: crypto/md5\n  File: internal/auth/hash.go:4\n```\n\n### SCA — Software Composition Analysis\n\nAnalyse les dépendances directes et transitives à la recherche de CVEs.\n\n```bash\n# Nancy (Go modules)\ngo list -json -m all | nancy sleuth\n\n# npm audit (Node.js)\nnpm audit --audit-level=high\n\n# pip-audit (Python)\npip-audit -r requirements.txt\n```\n\nRésultat type :\n\n```\nCVE-2024-12345  HIGH  github.com/some/lib v1.2.3\n  Affected: <1.2.4\n  Fix: upgrade to v1.2.4\n```\n\n### Politique de blocage\n\n```yaml\n# Bloquer la MR si vulnérabilité critique\nvariables:\n  DS_EXCLUDED_ANALYZERS: \"\"\n  DS_MAX_DEPTH: \"5\"\n\ndependency_scanning:\n  allow_failure: false   # bloque le pipeline si CVE critique\n```\n"
    },
    {
     "name": "Supply chain security",
     "type": "text",
     "skills": [
      "devsecops",
      "securite"
     ],
     "content": "## Supply chain security\n\nL'attaque de la supply chain consiste à compromettre une dépendance ou un outil de build pour infecter les utilisateurs en aval (ex: SolarWinds, Log4Shell, xz-utils).\n\n### SBOM — Software Bill of Materials\n\nUn SBOM est un inventaire exhaustif de tous les composants d'une application.\n\n```bash\n# Syft — générer un SBOM\nsyft scan . -o spdx-json > sbom.spdx.json\n\n# Grype — scanner un SBOM pour des CVEs\ngrype sbom:sbom.spdx.json\n```\n\n```json\n{\n  \"spdxVersion\": \"SPDX-2.3\",\n  \"packages\": [\n    {\n      \"name\": \"github.com/go-chi/chi\",\n      \"versionInfo\": \"v5.1.0\",\n      \"licenseConcluded\": \"MIT\"\n    }\n  ]\n}\n```\n\n### Sigstore / Cosign — signer les images\n\n```bash\n# Signer une image après le build\ncosign sign --key cosign.key registry.example.com/myapp:v1.2.3\n\n# Vérifier la signature avant le déploiement\ncosign verify --key cosign.pub registry.example.com/myapp:v1.2.3\n```\n\n### Dépendances épinglées vs flottantes\n\n```go\n// go.sum — hash SHA256 de chaque module\n// Garantit que tu télécharges exactement ce qui a été validé\ngithub.com/go-chi/chi/v5 v5.1.0 h1:acT6+...\ngithub.com/go-chi/chi/v5 v5.1.0/go.mod h1:DmT+...\n```\n\n```json\n// package-lock.json — équivalent npm\n// TOUJOURS commiter le lock file\n{\n  \"packages\": {\n    \"node_modules/express\": {\n      \"version\": \"4.19.2\",\n      \"integrity\": \"sha512-...\",\n      \"resolved\": \"https://registry.npmjs.org/express/-/express-4.19.2.tgz\"\n    }\n  }\n}\n```\n\nNe jamais utiliser `latest` comme tag d'image dans un pipeline de production.\n"
    },
    {
     "name": "Quiz — DevSecOps",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "shift-left-security",
      "sast-et-analyse-de-dépendances",
      "supply-chain-security"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Qu'est-ce que le 'shift-left security' ?",
       "answers": [
        {
         "id": "a",
         "text": "Intégrer les contrôles de sécurité le plus tôt possible dans le cycle de développement",
         "correct": true
        },
        {
         "id": "b",
         "text": "Confier la sécurité à une équipe dédiée séparée des développeurs",
         "correct": false
        },
        {
         "id": "c",
         "text": "Déployer la sécurité en priorité sur les environnements de gauche (dev/staging)",
         "correct": false
        },
        {
         "id": "d",
         "text": "Effectuer les tests de sécurité uniquement avant la mise en production",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quelle est la différence entre SAST et DAST ?",
       "answers": [
        {
         "id": "a",
         "text": "SAST analyse le code statique, DAST teste l'application en cours d'exécution",
         "correct": true
        },
        {
         "id": "b",
         "text": "SAST est pour les serveurs, DAST pour les bases de données",
         "correct": false
        },
        {
         "id": "c",
         "text": "SAST est automatique, DAST est manuel",
         "correct": false
        },
        {
         "id": "d",
         "text": "SAST détecte les secrets, DAST détecte les CVEs",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Pourquoi faut-il toujours commiter le fichier lock (package-lock.json, go.sum…) ?",
       "answers": [
        {
         "id": "a",
         "text": "Il garantit que tout le monde installe exactement les mêmes versions vérifiées",
         "correct": true
        },
        {
         "id": "b",
         "text": "Il accélère l'installation des dépendances",
         "correct": false
        },
        {
         "id": "c",
         "text": "Il contient les licences des dépendances",
         "correct": false
        },
        {
         "id": "d",
         "text": "Il est requis par les outils CI/CD pour fonctionner",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "distrobox",
  "spec": {
   "title": "Distrobox Creation and Application Export",
   "description": "Crée une Distrobox, installe xcowfortune et exporte l'application vers ton menu d'applications.",
   "category": "devops",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Maître Distrobox",
    "icon": "📦"
   },
   "modules": [
    {
     "name": "Distrobox Creation and Application Export",
     "type": "lab",
     "checkProvider": "local",
     "steps": [
      {
       "title": "Distrobox 'lab' créé",
       "checkType": "distrobox_lab3_container",
       "checkParams": {
        "container_name": "lab"
       }
      },
      {
       "title": "xcowfortune installé dans la Distrobox",
       "checkType": "distrobox_lab3_install",
       "checkParams": {
        "container_name": "lab",
        "app": "xcowfortune"
       }
      },
      {
       "title": "Application xcowfortune exportée",
       "checkType": "distrobox_lab3_export",
       "checkParams": {
        "container_name": "lab",
        "app": "xcowfortune"
       }
      }
     ],
     "skills": [
      "linux",
      "containers"
     ],
     "content": "## Objectifs\n\nApprendre à :\n\n- Créer un Distrobox à partir d'une image par défaut\n- Installer une application (`xcowfortune`) à l'intérieur du Distrobox\n- Exécuter l'application et voir un affichage graphique en dehors du terminal\n- Exporter l'application pour la rendre directement accessible depuis le menu des applications de votre workstation\n\n---\n\n## Contexte\n\nDistrobox est un outil qui permet d'exécuter une distribution Linux complète au sein de votre environnement de bureau actuel, sans avoir besoin de virtualisation ou de dual-boot. C'est particulièrement utile pour tester des applications dans un autre environnement Linux ou pour isoler des dépendances logicielles.\n\nDans ce lab, vous allez créer une Distrobox, installer une application simple, l'exécuter et l'exporter pour qu'elle soit utilisable directement depuis le menu d'applications de votre workstation.\n\n---\n\n## Étapes détaillées\n\n### 1. Créer un Distrobox\n\nCréez une Distrobox à partir de l'image par défaut.\n\n```bash\ndistrobox create --name lab\n```\n\nCette commande créera un Distrobox nommé `lab` à partir de l'image Fedora la plus récente.\n\n<!-- check:0 -->\n\n### 2. Installer xcowfortune\n\nUne fois la Distrobox créé, démarrez-le et installez le paquet `xcowfortune` à l'aide du gestionnaire de paquets `dnf`.\n\n```bash\ndistrobox enter lab\n```\n\nEnsuite, installez l'application :\n\n```bash\nsudo dnf install xcowfortune\n```\n\n<!-- check:1 -->\n\n### 3. Exécuter l'application\n\nDepuis la Distrobox, lancez l'application `xcowfortune` :\n\n```bash\nxcowfortune\n```\n\nUne fenêtre graphique doit apparaître avec un message aléatoire et une vache.\n\nCela montre comment vous pouvez exécuter des applications graphiques depuis une Distrobox et les voir directement sur votre bureau.\n\n### 4. Exporter l'application\n\nMaintenant, exportez l'application `xcowfortune` pour qu'elle devienne accessible directement depuis le menu des applications de votre workstation.\n\n```bash\ndistrobox-export -a xcowfortune\n```\n\nCette commande exporte l'application, la rendant accessible depuis votre environnement de bureau comme si elle était installée nativement.\n\n<!-- check:2 -->\n\n### Nettoyage *(optionnel)*\n\nPour supprimer l'application exportée ultérieurement :\n\n```bash\ndistrobox-export --delete -a xcowfortune\n```\n\n---\n\n## Félicitations !\n\nVous avez :\n\n- Créé une Distrobox\n- Installé une application à l'intérieur\n- Exécuté une application graphique depuis la Distrobox\n- Exporté l'application vers votre environnement de bureau\n\n---\n\n## Ressources\n\n| Ressource | Lien |\n|---|---|\n| Documentation Distrobox | distrobox.it |\n| GitHub Distrobox | github.com/89luca89/distrobox |\n"
    },
    {
     "name": "Quiz Distrobox",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "distrobox-creation-and-application-export"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quel est le rôle principal de Distrobox ?",
       "answers": [
        {
         "id": "a",
         "text": "Faire tourner n'importe quelle distribution Linux dans un conteneur avec une intégration complète avec l'hôte",
         "correct": true
        },
        {
         "id": "b",
         "text": "Construire et publier des images Docker",
         "correct": false
        },
        {
         "id": "c",
         "text": "Gérer des machines virtuelles sous Linux",
         "correct": false
        },
        {
         "id": "d",
         "text": "Orchestrer des conteneurs sur plusieurs nœuds",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quelle commande crée une Distrobox nommée 'mybox' à partir de l'image Ubuntu ?",
       "answers": [
        {
         "id": "a",
         "text": "distrobox create --name mybox --image ubuntu",
         "correct": true
        },
        {
         "id": "b",
         "text": "distrobox run --name mybox ubuntu",
         "correct": false
        },
        {
         "id": "c",
         "text": "distrobox new ubuntu mybox",
         "correct": false
        },
        {
         "id": "d",
         "text": "distrobox start mybox ubuntu",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Que fait la commande distrobox-export --app ?",
       "answers": [
        {
         "id": "a",
         "text": "Exporte une application graphique du conteneur vers le bureau de l'hôte",
         "correct": true
        },
        {
         "id": "b",
         "text": "Exporte le conteneur sous forme d'archive compressée",
         "correct": false
        },
        {
         "id": "c",
         "text": "Copie le système de fichiers du conteneur vers l'hôte",
         "correct": false
        },
        {
         "id": "d",
         "text": "Crée une image Docker à partir de la Distrobox",
         "correct": false
        }
       ]
      },
      {
       "id": "q4",
       "type": "single",
       "question": "Quelle commande permet d'entrer dans une Distrobox existante nommée 'mybox' ?",
       "answers": [
        {
         "id": "a",
         "text": "distrobox enter mybox",
         "correct": true
        },
        {
         "id": "b",
         "text": "distrobox run mybox",
         "correct": false
        },
        {
         "id": "c",
         "text": "distrobox shell mybox",
         "correct": false
        },
        {
         "id": "d",
         "text": "distrobox exec mybox bash",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "docker-fundamentals",
  "spec": {
   "title": "Les fondamentaux de Docker",
   "description": "Comprendre les conteneurs de A à Z — images, volumes, réseau et Docker Compose.",
   "category": "devops",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Capitaine Docker",
    "icon": "🐳"
   },
   "modules": [
    {
     "name": "Images et conteneurs",
     "type": "text",
     "skills": [
      "docker"
     ],
     "content": "## Images et conteneurs\n\nUn **conteneur** est une instance en cours d'exécution d'une **image**.\nUne image est un instantané de système de fichiers en lecture seule, construit en couches.\n\n```bash\ndocker pull nginx:alpine          # télécharger une image\ndocker images                     # lister les images locales\ndocker run -d -p 8080:80 nginx    # lancer en arrière-plan, mapper le port\ndocker ps                         # lister les conteneurs en cours\ndocker ps -a                      # tous les conteneurs (arrêtés inclus)\ndocker logs <id>                  # afficher les logs\ndocker exec -it <id> sh           # ouvrir un shell dans le conteneur\ndocker stop <id>                  # arrêt propre\ndocker rm <id>                    # supprimer un conteneur arrêté\n```\n\n### Dockerfile\n\n```dockerfile\nFROM node:22-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --omit=dev\nCOPY . .\nEXPOSE 3000\nCMD [\"node\", \"server.js\"]\n```\n\n```bash\ndocker build -t monapp:1.0 .\ndocker run -p 3000:3000 monapp:1.0\n```\n"
    },
    {
     "name": "Volumes et réseau",
     "type": "text",
     "skills": [
      "docker"
     ],
     "content": "## Volumes et réseau\n\n### Volumes (stockage persistant)\n\n```bash\ndocker volume create mesdonnees\ndocker run -v mesdonnees:/data postgres\ndocker run -v $(pwd):/app monapp    # montage de répertoire\n```\n\n### Réseaux\n\n```bash\ndocker network create monreseau\ndocker run --network monreseau --name db postgres\ndocker run --network monreseau --name app monapp\n# le conteneur \"app\" peut joindre \"db\" par son nom d'hôte\n```\n"
    },
    {
     "name": "Docker Compose",
     "type": "text",
     "skills": [
      "docker"
     ],
     "content": "## Docker Compose\n\nDéfinir une application multi-conteneurs dans un seul fichier `compose.yaml` :\n\n```yaml\nservices:\n  db:\n    image: postgres:16\n    environment:\n      POSTGRES_PASSWORD: secret\n    volumes:\n      - pgdata:/var/lib/postgresql/data\n\n  app:\n    build: .\n    ports:\n      - \"3000:3000\"\n    depends_on:\n      - db\n    environment:\n      DATABASE_URL: postgres://postgres:secret@db:5432/app\n\nvolumes:\n  pgdata:\n```\n\n```bash\ndocker compose up -d      # démarrer en arrière-plan\ndocker compose logs -f    # suivre les logs\ndocker compose down -v    # arrêter et supprimer les volumes\n```\n"
    },
    {
     "name": "Quiz Docker Fundamentals",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "images-et-conteneurs",
      "volumes-et-réseau",
      "docker-compose"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle commande construit une image Docker à partir du Dockerfile du répertoire courant ?",
       "answers": [
        {
         "id": "a",
         "text": "docker build .",
         "correct": true
        },
        {
         "id": "b",
         "text": "docker create .",
         "correct": false
        },
        {
         "id": "c",
         "text": "docker run .",
         "correct": false
        },
        {
         "id": "d",
         "text": "docker compile .",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quel est le rôle principal d'un volume Docker ?",
       "answers": [
        {
         "id": "a",
         "text": "Persister des données indépendamment du cycle de vie du conteneur",
         "correct": true
        },
        {
         "id": "b",
         "text": "Exposer les ports du conteneur vers l'hôte",
         "correct": false
        },
        {
         "id": "c",
         "text": "Réduire la taille de l'image",
         "correct": false
        },
        {
         "id": "d",
         "text": "Partager des variables d'environnement entre conteneurs",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quel flag lance un conteneur en mode détaché (arrière-plan) ?",
       "answers": [
        {
         "id": "a",
         "text": "-d",
         "correct": true
        },
        {
         "id": "b",
         "text": "-b",
         "correct": false
        },
        {
         "id": "c",
         "text": "--background",
         "correct": false
        },
        {
         "id": "d",
         "text": "-r",
         "correct": false
        }
       ]
      },
      {
       "id": "q4",
       "type": "single",
       "question": "Quel fichier Docker Compose lit-il par défaut pour définir une application multi-conteneurs ?",
       "answers": [
        {
         "id": "a",
         "text": "docker-compose.yml",
         "correct": true
        },
        {
         "id": "b",
         "text": "Dockerfile",
         "correct": false
        },
        {
         "id": "c",
         "text": "compose.json",
         "correct": false
        },
        {
         "id": "d",
         "text": "services.yaml",
         "correct": false
        }
       ]
      },
      {
       "id": "q5",
       "type": "single",
       "question": "Que fait l'instruction EXPOSE dans un Dockerfile ?",
       "answers": [
        {
         "id": "a",
         "text": "Documente le port sur lequel le conteneur écoute à l'exécution",
         "correct": true
        },
        {
         "id": "b",
         "text": "Publie automatiquement le port vers l'hôte",
         "correct": false
        },
        {
         "id": "c",
         "text": "Ouvre une règle pare-feu sur l'hôte",
         "correct": false
        },
        {
         "id": "d",
         "text": "Lance le processus du conteneur sur ce port",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "git-advanced",
  "spec": {
   "title": "Git avancé et workflows",
   "description": "Va au-delà du git commit : rebase interactif, gestion des branches, GitFlow, hooks et collaboration en équipe. Pour les développeurs qui veulent maîtriser leur historique.",
   "category": "devops",
   "difficulty": "intermediate",
   "public": false,
   "badge": {
    "name": "Maître Git",
    "icon": "🌿"
   },
   "modules": [
    {
     "name": "Rappels Git essentiels",
     "type": "text",
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/01-intro.md"
    },
    {
     "name": "Branches et stratégies de fusion",
     "type": "text",
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/02-images.md",
     "prerequisites": [
      "rappels-git-essentiels"
     ]
    },
    {
     "name": "Rebase interactif",
     "type": "text",
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/03-networking.md",
     "prerequisites": [
      "branches-et-stratégies-de-fusion"
     ]
    },
    {
     "name": "GitFlow et trunk-based development",
     "type": "text",
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/04-compose.md",
     "prerequisites": [
      "rebase-interactif"
     ]
    },
    {
     "name": "Hooks et automatisation",
     "type": "text",
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/04-compose.md",
     "prerequisites": [
      "gitflow-et-trunk-based-development"
     ]
    },
    {
     "name": "Quiz Git Avancé",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "rappels-git-essentiels",
      "branches-et-stratégies-de-fusion",
      "rebase-interactif",
      "gitflow-et-trunk-based-development",
      "hooks-et-automatisation"
     ],
     "passingScore": 80,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle commande permet de réécrire l'historique des 3 derniers commits ?",
       "answers": [
        {
         "id": "a",
         "text": "git rebase -i HEAD~3",
         "correct": true
        },
        {
         "id": "b",
         "text": "git reset HEAD~3",
         "correct": false
        },
        {
         "id": "c",
         "text": "git amend -3",
         "correct": false
        },
        {
         "id": "d",
         "text": "git rewrite HEAD~3",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Dans GitFlow, quelle branche reçoit les hotfixes ?",
       "answers": [
        {
         "id": "a",
         "text": "main et develop",
         "correct": true
        },
        {
         "id": "b",
         "text": "feature",
         "correct": false
        },
        {
         "id": "c",
         "text": "release",
         "correct": false
        },
        {
         "id": "d",
         "text": "staging",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quel fichier configure les hooks côté client ?",
       "answers": [
        {
         "id": "a",
         "text": ".git/hooks/",
         "correct": true
        },
        {
         "id": "b",
         "text": ".gitconfig",
         "correct": false
        },
        {
         "id": "c",
         "text": ".githooks",
         "correct": false
        },
        {
         "id": "d",
         "text": "hooks.yaml",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "gitlab-cicd",
  "spec": {
   "title": "GitLab Branching & CI/CD",
   "description": "Réalise ta première Merge Request GitLab avec un pipeline CI/CD et des commits conventionnels.",
   "category": "devops",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Maître CI/CD",
    "icon": "🚀"
   },
   "modules": [
    {
     "name": "Introduction au CI/CD avec GitLab",
     "type": "text",
     "skills": [
      "gitlab",
      "cicd"
     ],
     "content": "## Qu'est-ce que le CI/CD ?\n\nLe **CI/CD** (Intégration Continue / Déploiement Continu) est une pratique DevOps qui automatise la vérification et la livraison du code à chaque modification.\n\n- **CI — Intégration Continue** : à chaque push, une pipeline s'exécute automatiquement pour compiler, tester et valider le code.\n- **CD — Déploiement Continu** : si la CI passe, le code est automatiquement livré en production (ou en staging).\n\nL'objectif est de détecter les problèmes **le plus tôt possible**, avant qu'ils n'atteignent la production.\n\n---\n\n## Le workflow GitLab\n\nGitLab organise le travail autour de trois concepts clés :\n\n### 1. Les branches\nOn ne travaille jamais directement sur `main`. Chaque fonctionnalité ou correction se développe sur une **branche dédiée**, isolée du reste.\n\n```\nmain\n└── feature/add-greeting   ← ta branche de travail\n```\n\n### 2. Les Merge Requests (MR)\nUne fois le travail terminé, on ouvre une **Merge Request** : une demande de fusion de ta branche vers `main`. C'est là que l'équipe peut relire le code et que la pipeline CI/CD se déclenche automatiquement.\n\n### 3. La pipeline `.gitlab-ci.yml`\nCe fichier à la racine du projet définit les étapes automatiques qui s'exécutent à chaque push :\n\n```yaml\nstages:\n  - test\n\nrun-tests:\n  stage: test\n  script:\n    - python lab.py\n```\n\nSi la pipeline passe (✅), la MR est prête à être fusionnée.\n\n---\n\n## Les Conventional Commits\n\nPour garder un historique lisible, on utilise une convention de nommage pour les commits et les branches.\n\n### Format d'un commit conventionnel\n```\nfeat: add user greeting function\nfix: correct output format\ndocs: update README\n```\n\nLes types courants : `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`.\n\n### Format d'une branche conventionnelle\n```\nfeature/add-greeting\nfix/wrong-output\n```\n\n---\n\n## Pourquoi c'est important ?\n\n| Sans CI/CD | Avec CI/CD |\n|---|---|\n| Les bugs sont découverts en production | Les bugs sont détectés au push |\n| Le code est intégré rarement (conflits) | Le code est intégré en continu |\n| Les déploiements sont manuels et risqués | Les déploiements sont automatisés et fiables |\n| L'historique git est illisible | L'historique est structuré et exploitable |\n\n---\n\nPrêt ? 🚀\n"
    },
    {
     "name": "GitLab Branching & CI/CD with Conventional Commits",
     "type": "lab",
     "src": "http://10.89.0.1:8880/e-learning/devops-course.git",
     "ref": "main",
     "path": "modules/lab1/content.md",
     "labUrl": "http://localhost:8880/e-learning/devops-course/-/blob/main/modules/lab1/content.md",
     "checkProvider": "gitlab",
     "steps": [
      {
       "title": "Commit conventionnel effectué",
       "checkType": "gitlab_conventional_commit_on_branch",
       "checkParams": {
        "project": "e-learning/{{ .Username }}",
        "pattern": "feature/"
       }
      },
      {
       "title": "Modifications poussées",
       "checkType": "gitlab_commit_on_branch",
       "checkParams": {
        "project": "e-learning/{{ .Username }}",
        "pattern": "feature/"
       }
      },
      {
       "title": "Merge Request ouverte",
       "checkType": "gitlab_mr_open",
       "checkParams": {
        "project": "e-learning/{{ .Username }}"
       }
      },
      {
       "title": "Pipeline CI/CD passée",
       "checkType": "gitlab_pipeline_passed",
       "checkParams": {
        "project": "e-learning/{{ .Username }}"
       }
      }
     ]
    },
    {
     "name": "Quiz de validation",
     "type": "quiz",
     "inline": false,
     "passingScore": 70,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "points": 1,
       "question": "Qu'est-ce qu'une Merge Request dans GitLab ?",
       "answers": [
        {
         "id": "a",
         "text": "Une demande de fusion de branches",
         "correct": true
        },
        {
         "id": "b",
         "text": "Un type de pipeline",
         "correct": false
        },
        {
         "id": "c",
         "text": "Un outil de monitoring",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "harbor-podman",
  "spec": {
   "title": "Harbor Searching & Podman Running",
   "description": "Recherche une image sur Harbor et exécute un conteneur nginx avec Podman.",
   "category": "devops",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Gardien du Registre",
    "icon": "⚓"
   },
   "modules": [
    {
     "name": "Harbor Searching & Podman Running",
     "type": "lab",
     "checkProvider": "local",
     "steps": [
      {
       "title": "Image nginx téléchargée",
       "checkType": "podman_images",
       "checkParams": {
        "image": "nginx"
       }
      },
      {
       "title": "Conteneur my-nginx démarré",
       "checkType": "podman_lab2",
       "checkParams": {
        "image": "nginx",
        "container_name": "my-nginx"
       }
      }
     ],
     "skills": [
      "docker",
      "containers"
     ],
     "content": "## Objectifs\n\nApprendre à :\n\n- Rechercher des images Docker sur Harbor (interface web du registre)\n- Tirer (pull) une image avec Podman (CLI conteneurs)\n- Exécuter un conteneur avec des options spécifiques :\n  - `-it` → Mode Interactif Terminal\n  - `--name` → Donner un nom personnalisé\n  - `--rm` → Supprimer automatiquement à la sortie\n- Valider que le conteneur est en cours d'exécution\n\n> **Tips :** Besoin d'aide sur une commande Podman ? Utilise `man podman-run` (page de manuel en CLI)\n\n---\n\n## Contexte\n\n| Composant | Rôle | Type |\n|---|---|---|\n| Harbor | Stocker et gérer les images de conteneurs | Interface Web |\n| Podman | Gérer les conteneurs (compatible Docker) | CLI |\n\n> **Pourquoi Podman ?** Il ne nécessite pas de privilèges root (`sudo`), ce qui le rend plus sûr pour les postes de travail.\n\n---\n\n## Étapes détaillées\n\n### 1. Accéder à Harbor DMZ\n\nOuvre ton navigateur et accède à l'instance Harbor.\n\n> **Tips :** Trouve l'URL dans les favoris Firefox.\n\n### 2. Rechercher l'image nginx\n\nUtilise la barre de recherche en haut de Harbor :\n\n```\ndocker.io/library/nginx\n```\n\nSi l'image n'est pas sur la première page → utilise les boutons de pagination en bas.\n\n### 3. Tirer (pull) l'image\n\nClique sur l'image → copie la commande de pull via le bouton de copie.\n\nExécute dans le terminal :\n\n```bash\npodman pull docker.io/library/nginx:latest\n```\n\nTélécharge la dernière version de l'image nginx sur ta machine.\n\n<!-- check:0 -->\n\n### 4. Exécuter le conteneur\n\nExécute avec ces options :\n\n```bash\npodman run -it --name my-nginx --rm docker.io/library/nginx:latest\n```\n\n| Option | Signification | Utilité |\n|---|---|---|\n| `-it` | Interactif Terminal | Ouvre un shell dans le conteneur |\n| `--name my-nginx` | — | Assigne un nom lisible |\n| `--rm` | supprimer | Supprime automatiquement le conteneur à la sortie |\n\nLance un conteneur interactif `my-nginx`. Sors avec **CTRL + C**.\n\n<!-- check:1 -->\n\n### 5. Valider\n\nOuvre un nouveau terminal (**CTRL + MAJ + T**) et exécute :\n\n```bash\npodman ps -a\n```\n\nListe tous les conteneurs (en cours + arrêtés). Cherche `my-nginx`.\n\n---\n\n## Bravo !\n\nTu as réussi à :\n\n- Rechercher une image sur Harbor\n- La tirer avec Podman\n- L'exécuter avec des options personnalisées\n- Vérifier que le conteneur fonctionne\n\n---\n\n## Ressources\n\n| Ressource | Lien |\n|---|---|\n| Docs Podman | docs.podman.io |\n| Manuel Podman | `man podman-run` dans le terminal |\n"
    },
    {
     "name": "Quiz Harbor & Podman",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "harbor-searching---podman-running"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Qu'est-ce que Harbor ?",
       "answers": [
        {
         "id": "a",
         "text": "Un registre d'images de conteneurs open-source avec gestion de la sécurité et des accès",
         "correct": true
        },
        {
         "id": "b",
         "text": "Un runtime de conteneurs alternatif à Docker",
         "correct": false
        },
        {
         "id": "c",
         "text": "Un plugin réseau pour Kubernetes",
         "correct": false
        },
        {
         "id": "d",
         "text": "Un outil de pipeline CI/CD",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quelle commande Podman télécharge une image depuis un registre ?",
       "answers": [
        {
         "id": "a",
         "text": "podman pull <image>",
         "correct": true
        },
        {
         "id": "b",
         "text": "podman fetch <image>",
         "correct": false
        },
        {
         "id": "c",
         "text": "podman get <image>",
         "correct": false
        },
        {
         "id": "d",
         "text": "podman download <image>",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quelle est la principale différence entre Podman et Docker par défaut ?",
       "answers": [
        {
         "id": "a",
         "text": "Podman fonctionne sans daemon et est rootless par défaut",
         "correct": true
        },
        {
         "id": "b",
         "text": "Podman ne supporte que les conteneurs Windows",
         "correct": false
        },
        {
         "id": "c",
         "text": "Podman ne peut pas construire des images",
         "correct": false
        },
        {
         "id": "d",
         "text": "Podman nécessite un cluster Kubernetes pour fonctionner",
         "correct": false
        }
       ]
      },
      {
       "id": "q4",
       "type": "single",
       "question": "Que montre la commande podman ps -a ?",
       "answers": [
        {
         "id": "a",
         "text": "Tous les conteneurs, y compris ceux qui sont arrêtés",
         "correct": true
        },
        {
         "id": "b",
         "text": "Uniquement les conteneurs en cours d'exécution",
         "correct": false
        },
        {
         "id": "c",
         "text": "Toutes les images disponibles sur le système",
         "correct": false
        },
        {
         "id": "d",
         "text": "Tous les volumes Podman",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "infrastructure-as-code",
  "spec": {
   "title": "Infrastructure as Code",
   "description": "Provisionner et versionner ton infrastructure comme du code avec Terraform et Pulumi — reproductibilité, collaboration et déploiements sans surprise.",
   "category": "devops",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Architecte IaC",
    "icon": "🏗️"
   },
   "modules": [
    {
     "name": "Principes de l'IaC",
     "type": "text",
     "skills": [
      "terraform",
      "iac"
     ],
     "content": "## Principes de l'IaC\n\nL'**Infrastructure as Code** consiste à décrire l'infrastructure (serveurs, réseaux, DNS, bases de données…) dans des fichiers texte versionnés, plutôt que de la configurer manuellement via une interface.\n\n### Avantages\n\n| Manuel (ClickOps) | Infrastructure as Code |\n|---|---|\n| Non reproductible | Reproductible à l'identique |\n| Pas de diff/review | Pull request, code review |\n| Erreurs humaines | Validé avant apply |\n| Undocumenté | Code = documentation |\n| Un seul env difficile à dupliquer | Multi-env trivial |\n\n### Concepts clés\n\n**Idempotence** : appliquer le même code deux fois donne le même résultat. Pas d'effets de bord.\n\n```bash\nterraform apply   # crée les ressources si elles n'existent pas\nterraform apply   # ne fait rien si l'état correspond déjà\n```\n\n**Infrastructure immuable** : au lieu de modifier un serveur existant, on en crée un nouveau et on détruit l'ancien.\n\n```\nv1 → déploiement du serveur A\nv2 → déploiement du serveur B + destruction du serveur A\n```\n\n### Les outils majeurs\n\n| Outil | Langage | Cloud |\n|---|---|---|\n| **Terraform** | HCL | Multi-cloud |\n| **Pulumi** | TypeScript/Go/Python | Multi-cloud |\n| **OpenTofu** | HCL (fork libre de Terraform) | Multi-cloud |\n| **CloudFormation** | JSON/YAML | AWS uniquement |\n| **Bicep** | DSL | Azure uniquement |\n"
    },
    {
     "name": "Terraform — les bases",
     "type": "text",
     "skills": [
      "terraform",
      "iac"
     ],
     "content": "## Terraform — les bases\n\n### Structure d'un projet\n\n```\ninfra/\n├── main.tf          # ressources\n├── variables.tf     # déclaration des variables\n├── outputs.tf       # valeurs exposées après apply\n├── terraform.tfvars # valeurs des variables (gitignored si secrets)\n└── versions.tf      # contraintes de version\n```\n\n### Provider et ressource\n\n```hcl\n# versions.tf\nterraform {\n  required_providers {\n    kubernetes = {\n      source  = \"hashicorp/kubernetes\"\n      version = \"~> 2.30\"\n    }\n  }\n}\n\n# main.tf\nprovider \"kubernetes\" {\n  config_path = \"~/.kube/config\"\n}\n\nresource \"kubernetes_namespace\" \"app\" {\n  metadata {\n    name = var.namespace\n  }\n}\n```\n\n### Variables et outputs\n\n```hcl\n# variables.tf\nvariable \"namespace\" {\n  type        = string\n  description = \"Kubernetes namespace cible\"\n  default     = \"production\"\n}\n\n# outputs.tf\noutput \"namespace_name\" {\n  value = kubernetes_namespace.app.metadata[0].name\n}\n```\n\n### Workflow\n\n```bash\nterraform init      # télécharger les providers\nterraform plan      # afficher les changements (dry-run)\nterraform apply     # appliquer\nterraform destroy   # tout supprimer\n```\n\nLe `plan` est essentiel : il montre exactement ce qui sera créé, modifié ou détruit — à lire attentivement avant l'apply.\n"
    },
    {
     "name": "State et collaboration",
     "type": "text",
     "skills": [
      "terraform",
      "iac"
     ],
     "content": "## State et collaboration\n\n### Le state Terraform\n\nTerraform maintient un fichier `terraform.tfstate` qui représente l'état actuel de l'infrastructure connue.\n\n```json\n{\n  \"resources\": [\n    {\n      \"type\": \"kubernetes_namespace\",\n      \"name\": \"app\",\n      \"instances\": [\n        {\n          \"attributes\": {\n            \"metadata\": [{\"name\": \"production\"}]\n          }\n        }\n      ]\n    }\n  ]\n}\n```\n\n> Ne jamais éditer `terraform.tfstate` manuellement. Ne jamais le commiter si il contient des secrets.\n\n### Remote state — collaboration en équipe\n\n```hcl\n# backend S3 (AWS)\nterraform {\n  backend \"s3\" {\n    bucket         = \"mon-terraform-state\"\n    key            = \"production/terraform.tfstate\"\n    region         = \"eu-west-3\"\n    dynamodb_table = \"terraform-locks\"  # verrou anti-concurrence\n    encrypt        = true\n  }\n}\n```\n\n```hcl\n# backend Kubernetes (pour un cluster local)\nterraform {\n  backend \"kubernetes\" {\n    secret_suffix = \"production\"\n    config_path   = \"~/.kube/config\"\n  }\n}\n```\n\n### GitLab CI/CD et Terraform\n\n```yaml\nstages:\n  - plan\n  - apply\n\nterraform:plan:\n  stage: plan\n  image: hashicorp/terraform:1.8\n  script:\n    - terraform init\n    - terraform plan -out=tfplan\n  artifacts:\n    paths:\n      - tfplan\n\nterraform:apply:\n  stage: apply\n  image: hashicorp/terraform:1.8\n  script:\n    - terraform init\n    - terraform apply tfplan\n  when: manual   # déclenché manuellement après review du plan\n  only:\n    - main\n```\n"
    },
    {
     "name": "Quiz — Infrastructure as Code",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "principes-de-l-iac",
      "terraform---les-bases",
      "state-et-collaboration"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Qu'est-ce que l'idempotence dans le contexte de l'IaC ?",
       "answers": [
        {
         "id": "a",
         "text": "Appliquer le même code plusieurs fois donne toujours le même résultat",
         "correct": true
        },
        {
         "id": "b",
         "text": "Le code peut s'exécuter en parallèle sans conflit",
         "correct": false
        },
        {
         "id": "c",
         "text": "L'infrastructure se reconfigure seule en cas de panne",
         "correct": false
        },
        {
         "id": "d",
         "text": "Les ressources sont créées de façon atomique",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "À quoi sert la commande terraform plan ?",
       "answers": [
        {
         "id": "a",
         "text": "Afficher les changements qui seront appliqués sans les effectuer",
         "correct": true
        },
        {
         "id": "b",
         "text": "Initialiser les providers Terraform",
         "correct": false
        },
        {
         "id": "c",
         "text": "Sauvegarder le state dans un backend distant",
         "correct": false
        },
        {
         "id": "d",
         "text": "Valider la syntaxe HCL uniquement",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Pourquoi utiliser un remote state avec verrou (DynamoDB ou équivalent) ?",
       "answers": [
        {
         "id": "a",
         "text": "Éviter que deux personnes appliquent des changements en même temps et corrompent le state",
         "correct": true
        },
        {
         "id": "b",
         "text": "Chiffrer automatiquement toutes les ressources",
         "correct": false
        },
        {
         "id": "c",
         "text": "Accélérer l'exécution de terraform apply",
         "correct": false
        },
        {
         "id": "d",
         "text": "Partager les variables entre plusieurs projets",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "kubernetes-basics",
  "spec": {
   "title": "Les bases de Kubernetes",
   "description": "Déployer et gérer des applications conteneurisées sur Kubernetes — Pods, Deployments, Services et ConfigMaps.",
   "category": "devops",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Pilote Kubernetes",
    "icon": "☸️"
   },
   "modules": [
    {
     "name": "Concepts fondamentaux",
     "type": "text",
     "skills": [
      "kubernetes"
     ],
     "content": "## Concepts fondamentaux\n\nKubernetes (K8s) orchestre des conteneurs sur un cluster de nœuds.\n\n| Objet | Rôle |\n|---|---|\n| **Pod** | Plus petite unité déployable — un ou plusieurs conteneurs |\n| **Deployment** | Gère les réplicas et les mises à jour progressives des Pods |\n| **Service** | Point réseau stable devant les Pods |\n| **ConfigMap** | Configuration non sensible sous forme de clés/valeurs |\n| **Secret** | Données sensibles (encodées en base64) |\n| **Namespace** | Cluster virtuel pour l'isolation |\n\n```bash\nkubectl get pods -n default\nkubectl describe pod <nom>\nkubectl logs <pod> -f\nkubectl exec -it <pod> -- sh\nkubectl apply -f manifest.yaml\nkubectl delete -f manifest.yaml\n```\n"
    },
    {
     "name": "Deployments et Services",
     "type": "text",
     "skills": [
      "kubernetes"
     ],
     "content": "## Deployments et Services\n\n### Deployment\n\n```yaml\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: monapp\nspec:\n  replicas: 3\n  selector:\n    matchLabels:\n      app: monapp\n  template:\n    metadata:\n      labels:\n        app: monapp\n    spec:\n      containers:\n      - name: monapp\n        image: monapp:1.0\n        ports:\n        - containerPort: 3000\n        resources:\n          requests:\n            cpu: 100m\n            memory: 128Mi\n          limits:\n            cpu: 500m\n            memory: 256Mi\n```\n\n### Service\n\n```yaml\napiVersion: v1\nkind: Service\nmetadata:\n  name: monapp\nspec:\n  selector:\n    app: monapp\n  ports:\n  - port: 80\n    targetPort: 3000\n  type: ClusterIP\n```\n"
    },
    {
     "name": "ConfigMaps et Secrets",
     "type": "text",
     "skills": [
      "kubernetes"
     ],
     "content": "## ConfigMaps et Secrets\n\n### ConfigMap\n\n```yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\ndata:\n  LOG_LEVEL: info\n  MAX_CONNECTIONS: \"100\"\n```\n\nRéférencer dans un Pod :\n```yaml\nenvFrom:\n- configMapRef:\n    name: app-config\n```\n\n### Secret\n\n```bash\nkubectl create secret generic db-creds \\\n  --from-literal=password=supersecret\n```\n\n```yaml\nenv:\n- name: DB_PASSWORD\n  valueFrom:\n    secretKeyRef:\n      name: db-creds\n      key: password\n```\n\n> Ne jamais commiter des manifests Secret avec des valeurs en clair dans git — utiliser Sealed Secrets ou un coffre-fort externe.\n"
    },
    {
     "name": "Quiz Kubernetes Basics",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "concepts-fondamentaux",
      "deployments-et-services",
      "configmaps-et-secrets"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle est la plus petite unité déployable dans Kubernetes ?",
       "answers": [
        {
         "id": "a",
         "text": "Pod",
         "correct": true
        },
        {
         "id": "b",
         "text": "Container",
         "correct": false
        },
        {
         "id": "c",
         "text": "Node",
         "correct": false
        },
        {
         "id": "d",
         "text": "ReplicaSet",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quelle ressource Kubernetes gère les mises à jour progressives et les rollbacks d'un ensemble de Pods ?",
       "answers": [
        {
         "id": "a",
         "text": "Deployment",
         "correct": true
        },
        {
         "id": "b",
         "text": "Service",
         "correct": false
        },
        {
         "id": "c",
         "text": "ConfigMap",
         "correct": false
        },
        {
         "id": "d",
         "text": "Namespace",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quel est le rôle principal d'un Service Kubernetes ?",
       "answers": [
        {
         "id": "a",
         "text": "Exposer un ensemble de Pods avec un point d'accès réseau stable",
         "correct": true
        },
        {
         "id": "b",
         "text": "Stocker des valeurs de configuration sensibles",
         "correct": false
        },
        {
         "id": "c",
         "text": "Planifier les Pods sur les nœuds",
         "correct": false
        },
        {
         "id": "d",
         "text": "Persister les données entre les redémarrages de Pods",
         "correct": false
        }
       ]
      },
      {
       "id": "q4",
       "type": "single",
       "question": "En quoi un Secret diffère-t-il d'un ConfigMap ?",
       "answers": [
        {
         "id": "a",
         "text": "Les Secrets sont encodés en base64 et destinés aux données sensibles",
         "correct": true
        },
        {
         "id": "b",
         "text": "Les Secrets sont stockés sur disque tandis que les ConfigMaps restent en mémoire",
         "correct": false
        },
        {
         "id": "c",
         "text": "Les Secrets ne peuvent être montés qu'en volumes, pas en variables d'environnement",
         "correct": false
        },
        {
         "id": "d",
         "text": "Il n'y a aucune différence, ce sont des ressources identiques",
         "correct": false
        }
       ]
      },
      {
       "id": "q5",
       "type": "single",
       "question": "Quelle commande liste tous les Pods du namespace courant ?",
       "answers": [
        {
         "id": "a",
         "text": "kubectl get pods",
         "correct": true
        },
        {
         "id": "b",
         "text": "kubectl list pods",
         "correct": false
        },
        {
         "id": "c",
         "text": "kubectl describe pods",
         "correct": false
        },
        {
         "id": "d",
         "text": "kubectl show pods",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "linux-intro",
  "spec": {
   "title": "Introduction à Linux",
   "description": "Apprends les bases de Linux : navigation, gestion de fichiers et permissions. Un cours pratique pour démarrer sur la bonne voie.",
   "category": "linux",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Explorateur Linux",
    "icon": "🐧"
   },
   "modules": [
    {
     "name": "Navigation et système de fichiers",
     "type": "text",
     "skills": [
      "linux"
     ],
     "content": "## Navigation et système de fichiers\n\nLinux organise tout dans une arborescence unique qui part de `/` (la racine).\n\n### Arborescence essentielle\n\n| Répertoire | Contenu |\n|---|---|\n| `/` | Racine du système |\n| `/home` | Répertoires personnels des utilisateurs |\n| `/etc` | Fichiers de configuration système |\n| `/var` | Données variables (logs, bases de données) |\n| `/tmp` | Fichiers temporaires (effacés au reboot) |\n| `/usr/bin` | Programmes installés |\n| `/proc` | Pseudo-FS : infos sur les processus en cours |\n\n### Commandes de navigation\n\n```bash\npwd                  # affiche le répertoire courant\nls -lah              # liste détaillée avec tailles humaines et fichiers cachés\ncd /etc              # aller dans /etc\ncd ~                 # aller dans ton home\ncd -                 # retourner au répertoire précédent\ntree -L 2            # arborescence sur 2 niveaux\n```\n\n### Manipulation de fichiers\n\n```bash\ncp fichier.txt backup/     # copier\nmv ancien.txt nouveau.txt  # renommer / déplacer\nrm -rf dossier/            # supprimer récursivement (attention !)\nmkdir -p a/b/c             # créer l'arborescence en une fois\nln -s /chemin/src lien     # créer un lien symbolique\nfind / -name \"*.conf\" 2>/dev/null  # chercher des fichiers\n```\n"
    },
    {
     "name": "Utilisateurs, groupes et permissions",
     "type": "text",
     "skills": [
      "linux"
     ],
     "content": "## Utilisateurs, groupes et permissions\n\nLinux est un système multi-utilisateurs. Chaque fichier appartient à un utilisateur et un groupe.\n\n### Lecture des permissions\n\n```\n-rwxr-xr--  1  alice  devops  4096  Jun 30  script.sh\n ↑↑↑ ↑↑↑ ↑↑↑\n │   │   └── autres : lecture seule (r--)\n │   └────── groupe  : lecture + exécution (r-x)\n └────────── propriétaire : tout (rwx)\n```\n\n| Lettre | Signification | Valeur octale |\n|---|---|---|\n| `r` | lecture | 4 |\n| `w` | écriture | 2 |\n| `x` | exécution | 1 |\n\n### Modifier les permissions\n\n```bash\nchmod 755 script.sh       # rwxr-xr-x (propriétaire : tout, reste : r+x)\nchmod +x script.sh        # ajouter le droit d'exécution\nchmod -R 644 /var/www/    # récursif : rw-r--r--\nchown alice:devops app/   # changer propriétaire et groupe\n```\n\n### Gestion des utilisateurs\n\n```bash\nwhoami                    # utilisateur courant\nid                        # uid, gid et groupes\nsudo commande             # exécuter en tant que root\nuseradd -m -s /bin/bash alice   # créer un utilisateur\nusermod -aG docker alice        # ajouter alice au groupe docker\npasswd alice                    # changer le mot de passe\n```\n"
    },
    {
     "name": "Processus, services et réseau",
     "type": "text",
     "skills": [
      "linux"
     ],
     "content": "## Processus, services et réseau\n\n### Gérer les processus\n\n```bash\nps aux                 # liste tous les processus\ntop                    # moniteur interactif (q pour quitter)\nhtop                   # version améliorée de top\nkill -9 1234           # tuer le processus PID 1234\nkillall nginx          # tuer tous les processus nginx\nnohup ./script.sh &    # lancer en arrière-plan, immunisé aux déconnexions\njobs                   # voir les tâches de la session\n```\n\n### systemd — gestion des services\n\n```bash\nsystemctl status nginx          # état du service\nsystemctl start nginx           # démarrer\nsystemctl stop nginx            # arrêter\nsystemctl restart nginx         # redémarrer\nsystemctl enable nginx          # démarrer au boot\nsystemctl disable nginx         # ne plus démarrer au boot\njournalctl -u nginx -f          # logs du service en temps réel\n```\n\n### Commandes réseau essentielles\n\n```bash\nip addr show           # interfaces réseau et adresses IP\nip route show          # table de routage\nss -tlnp               # ports en écoute (remplace netstat)\ncurl -I https://example.com    # en-têtes HTTP\nping -c 4 8.8.8.8      # tester la connectivité\ntraceroute 8.8.8.8     # tracer la route réseau\nnmap -sV localhost     # scanner les ports ouverts\n```\n\n### Fichiers de logs utiles\n\n```bash\n/var/log/syslog        # logs système généraux (Debian/Ubuntu)\n/var/log/auth.log      # authentifications et sudo\n/var/log/nginx/        # logs nginx\njournalctl -f          # tous les logs en temps réel\njournalctl --since \"1 hour ago\"   # dernière heure\n```\n"
    },
    {
     "name": "Quiz Introduction à Linux",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "navigation-et-système-de-fichiers",
      "utilisateurs--groupes-et-permissions",
      "processus--services-et-réseau"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle commande affiche le répertoire de travail courant ?",
       "answers": [
        {
         "id": "a",
         "text": "pwd",
         "correct": true
        },
        {
         "id": "b",
         "text": "cwd",
         "correct": false
        },
        {
         "id": "c",
         "text": "dir",
         "correct": false
        },
        {
         "id": "d",
         "text": "ls .",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quelles permissions chmod 755 applique-t-il sur un fichier ?",
       "answers": [
        {
         "id": "a",
         "text": "rwxr-xr-x (propriétaire : tout, groupe/autres : lecture+exécution)",
         "correct": true
        },
        {
         "id": "b",
         "text": "rwxrwxrwx (tous : tout)",
         "correct": false
        },
        {
         "id": "c",
         "text": "rw-r--r-- (propriétaire : lecture+écriture, autres : lecture)",
         "correct": false
        },
        {
         "id": "d",
         "text": "r-xr-xr-x (lecture+exécution pour tous, pas d'écriture)",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quelle commande active un service systemd pour qu'il démarre automatiquement au boot ?",
       "answers": [
        {
         "id": "a",
         "text": "systemctl enable <service>",
         "correct": true
        },
        {
         "id": "b",
         "text": "systemctl start <service>",
         "correct": false
        },
        {
         "id": "c",
         "text": "systemctl activate <service>",
         "correct": false
        },
        {
         "id": "d",
         "text": "service <service> autostart",
         "correct": false
        }
       ]
      },
      {
       "id": "q4",
       "type": "single",
       "question": "Que affiche ps aux ?",
       "answers": [
        {
         "id": "a",
         "text": "Tous les processus en cours pour tous les utilisateurs",
         "correct": true
        },
        {
         "id": "b",
         "text": "Uniquement les processus de l'utilisateur courant",
         "correct": false
        },
        {
         "id": "c",
         "text": "Les connexions réseau et les ports en écoute",
         "correct": false
        },
        {
         "id": "d",
         "text": "L'utilisation du disque par processus",
         "correct": false
        }
       ]
      },
      {
       "id": "q5",
       "type": "single",
       "question": "Quelle commande ajoute l'utilisateur alice au groupe docker ?",
       "answers": [
        {
         "id": "a",
         "text": "usermod -aG docker alice",
         "correct": true
        },
        {
         "id": "b",
         "text": "groupadd alice docker",
         "correct": false
        },
        {
         "id": "c",
         "text": "adduser docker alice",
         "correct": false
        },
        {
         "id": "d",
         "text": "chgrp docker alice",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "monitoring-observability",
  "spec": {
   "title": "Monitoring & Observabilité",
   "description": "Comprendre ce qui se passe dans tes systèmes en production — métriques, logs, traces et alerting avec Prometheus, Grafana et Loki.",
   "category": "devops",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Sentinelle des systèmes",
    "icon": "📊"
   },
   "modules": [
    {
     "name": "Les trois piliers de l'observabilité",
     "type": "text",
     "skills": [
      "monitoring"
     ],
     "content": "## Les trois piliers de l'observabilité\n\nL'observabilité, c'est la capacité à comprendre l'état interne d'un système depuis ses sorties externes.\n\n### Métriques\n\nDes valeurs numériques mesurées dans le temps.\n\n```\nhttp_requests_total{method=\"GET\", status=\"200\"} 1842\nhttp_requests_total{method=\"POST\", status=\"500\"} 3\ngo_goroutines 42\nprocess_resident_memory_bytes 52428800\n```\n\nIdéal pour : taux d'erreur, latence, saturation, trafic (les \"Golden Signals\").\n\n### Logs\n\nDes événements textuels horodatés.\n\n```json\n{\"time\":\"2026-06-26T10:23:01Z\",\"level\":\"ERROR\",\"msg\":\"database connection failed\",\"service\":\"api\",\"err\":\"connection refused\"}\n{\"time\":\"2026-06-26T10:23:02Z\",\"level\":\"INFO\",\"msg\":\"retrying connection\",\"attempt\":1}\n```\n\nIdéal pour : debug post-mortem, audit trail, erreurs détaillées.\n\n### Traces\n\nLe chemin d'une requête à travers les services (tracing distribué).\n\n```\nRequest 7f3a...\n├─ api-gateway        12ms\n│  ├─ user-service     4ms\n│  └─ course-service  31ms\n│     └─ postgres      8ms\n└─ total              47ms\n```\n\nIdéal pour : identifier les goulots d'étranglement dans une architecture microservices.\n\n### Les Golden Signals (SRE Google)\n\n| Signal | Question |\n|---|---|\n| **Traffic** | Combien de requêtes par seconde ? |\n| **Errors** | Quel pourcentage échoue ? |\n| **Latency** | Combien de temps chaque requête prend-elle ? |\n| **Saturation** | À quel point le système est-il proche de ses limites ? |\n"
    },
    {
     "name": "Prometheus & Grafana",
     "type": "text",
     "skills": [
      "monitoring"
     ],
     "content": "## Prometheus & Grafana\n\n### Prometheus — collecte des métriques\n\nPrometheus scrape (tire) les métriques depuis tes services toutes les N secondes.\n\n```yaml\n# prometheus.yml\nscrape_configs:\n  - job_name: 'api'\n    scrape_interval: 15s\n    static_configs:\n      - targets: ['api:8080']\n```\n\nTon service expose une route `/metrics` :\n\n```go\n// Go — avec le client Prometheus officiel\nimport \"github.com/prometheus/client_golang/prometheus/promhttp\"\n\nhttp.Handle(\"/metrics\", promhttp.Handler())\n```\n\n### PromQL — requêtes Prometheus\n\n```promql\n# Taux de requêtes par seconde sur 5 min\nrate(http_requests_total[5m])\n\n# Taux d'erreurs (status 5xx) en %\n100 * rate(http_requests_total{status=~\"5..\"}[5m])\n      / rate(http_requests_total[5m])\n\n# P99 de latence\nhistogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))\n```\n\n### Grafana — dashboards\n\nGrafana visualise les données de Prometheus (et d'autres sources).\n\n```yaml\n# Kubernetes — kube-prometheus-stack (Helm)\nhelm repo add prometheus-community https://prometheus-community.github.io/helm-charts\nhelm install monitoring prometheus-community/kube-prometheus-stack \\\n  --namespace monitoring \\\n  --create-namespace\n```\n\nAccès aux dashboards prédéfinis :\n- Kubernetes cluster overview\n- Node exporter (CPU, RAM, disque, réseau)\n- Pods et déploiements\n- Ingress (Nginx/Traefik)\n\n### Loki — logs centralisés\n\n```yaml\n# Promtail collecte les logs des pods et les envoie à Loki\n# LogQL — syntaxe similaire à PromQL\n{namespace=\"production\", app=\"api\"} |= \"ERROR\"\n| json\n| line_format \"{{.msg}} — {{.err}}\"\n```\n"
    },
    {
     "name": "Alerting et SLOs",
     "type": "text",
     "skills": [
      "monitoring"
     ],
     "content": "## Alerting et SLOs\n\n### SLI / SLO / SLA\n\n| Terme | Définition | Exemple |\n|---|---|---|\n| **SLI** | Service Level Indicator — métrique mesurée | Taux de succès HTTP |\n| **SLO** | Service Level Objective — cible interne | 99,9% de succès |\n| **SLA** | Service Level Agreement — engagement contractuel | 99,5% garanti (pénalités si non atteint) |\n\n### Alertmanager\n\n```yaml\n# Règle d'alerte Prometheus\ngroups:\n  - name: api.rules\n    rules:\n      - alert: HighErrorRate\n        expr: |\n          rate(http_requests_total{status=~\"5..\"}[5m])\n          / rate(http_requests_total[5m]) > 0.01\n        for: 5m\n        labels:\n          severity: critical\n        annotations:\n          summary: \"Taux d'erreur > 1% depuis 5 minutes\"\n          runbook: \"https://wiki.internal/runbooks/high-error-rate\"\n```\n\n```yaml\n# alertmanager.yml — routage des alertes\nroute:\n  receiver: slack-ops\n  group_wait: 30s\n  group_interval: 5m\n\nreceivers:\n  - name: slack-ops\n    slack_configs:\n      - api_url: \"https://hooks.slack.com/services/...\"\n        channel: \"#ops-alerts\"\n        title: \"{{ .GroupLabels.alertname }}\"\n        text: \"{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}\"\n```\n\n### Runbooks\n\nUn runbook est la procédure à suivre quand une alerte se déclenche.\n\n```markdown\n# Runbook : HighErrorRate\n\n## Symptômes\nTaux d'erreurs HTTP 5xx > 1% depuis 5 minutes.\n\n## Diagnostic\n1. Vérifier les logs : `kubectl logs -n production -l app=api --tail=100`\n2. Vérifier la base de données : `kubectl get pods -n data`\n3. Vérifier les déploiements récents : `kubectl rollout history deployment/api`\n\n## Remédiation\n- Si déploiement récent : `kubectl rollout undo deployment/api`\n- Si base de données : escalader vers l'équipe data\n```\n"
    },
    {
     "name": "Quiz — Monitoring & Observabilité",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "les-trois-piliers-de-l-observabilité",
      "prometheus---grafana",
      "alerting-et-slos"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Parmi les Golden Signals, lequel mesure le pourcentage de requêtes qui échouent ?",
       "answers": [
        {
         "id": "a",
         "text": "Errors",
         "correct": true
        },
        {
         "id": "b",
         "text": "Latency",
         "correct": false
        },
        {
         "id": "c",
         "text": "Saturation",
         "correct": false
        },
        {
         "id": "d",
         "text": "Traffic",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Comment Prometheus collecte-t-il les métriques ?",
       "answers": [
        {
         "id": "a",
         "text": "Il scrape (tire) les métriques depuis les services à intervalles réguliers",
         "correct": true
        },
        {
         "id": "b",
         "text": "Les services poussent leurs métriques vers Prometheus",
         "correct": false
        },
        {
         "id": "c",
         "text": "Il lit les fichiers de logs des services",
         "correct": false
        },
        {
         "id": "d",
         "text": "Via des webhooks déclenchés à chaque événement",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quelle est la différence entre un SLO et un SLA ?",
       "answers": [
        {
         "id": "a",
         "text": "Le SLO est un objectif interne, le SLA est un engagement contractuel avec pénalités",
         "correct": true
        },
        {
         "id": "b",
         "text": "Le SLO concerne la latence, le SLA concerne la disponibilité",
         "correct": false
        },
        {
         "id": "c",
         "text": "Le SLA est toujours plus strict que le SLO",
         "correct": false
        },
        {
         "id": "d",
         "text": "Il n'y a pas de différence, ce sont des synonymes",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "networking-basics",
  "spec": {
   "title": "Réseaux — Les fondamentaux",
   "description": "Comprendre TCP/IP, les protocoles essentiels, les pare-feux et la sécurité réseau pour tout développeur ou administrateur système.",
   "category": "networking",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Architecte réseau",
    "icon": "🌐"
   },
   "modules": [
    {
     "name": "Modèle TCP/IP et protocoles",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "content": "## Modèle TCP/IP et protocoles\n\nInternet fonctionne en couches — chaque couche a un rôle précis et communique uniquement avec les couches adjacentes.\n\n### Les 4 couches TCP/IP\n\n| Couche | Rôle | Protocoles |\n|---|---|---|\n| **Application** | Ce que voit l'utilisateur | HTTP, HTTPS, DNS, SSH, FTP |\n| **Transport** | Fiabilité, ports | TCP, UDP |\n| **Internet** | Routage entre réseaux | IP, ICMP, ARP |\n| **Réseau** | Transmission physique | Ethernet, Wi-Fi |\n\n### TCP vs UDP\n\n```\nTCP (fiable)                    UDP (rapide)\n────────────────────────────    ────────────────────────\nConnexion établie (handshake)   Pas de connexion\nAccusé de réception             Pas de garantie\nOrdre des paquets garanti       Ordre non garanti\n→ HTTP, SSH, bases de données   → DNS, streaming, jeux\n```\n\n### Ports importants\n\n| Port | Protocole | Usage |\n|---|---|---|\n| 22 | SSH | Administration distante chiffrée |\n| 80 | HTTP | Web non chiffré |\n| 443 | HTTPS | Web chiffré |\n| 53 | DNS | Résolution de noms |\n| 5432 | PostgreSQL | Base de données |\n| 6379 | Redis | Cache |\n\n```bash\n# Voir les ports ouverts sur ta machine\nss -tlnp\n\n# Tester si un port est ouvert\nnc -zv 10.0.0.1 443\n```\n"
    },
    {
     "name": "Adressage IP et sous-réseaux",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "content": "## Adressage IP et sous-réseaux\n\n### Adresses IPv4\n\nUne adresse IPv4 est composée de 4 octets (32 bits) : `192.168.1.100`\n\n#### Plages privées (non routables sur internet)\n\n| Plage | Usage courant |\n|---|---|\n| `10.0.0.0/8` | Réseaux d'entreprise |\n| `172.16.0.0/12` | Clusters (Kubernetes, Docker) |\n| `192.168.0.0/16` | Réseaux domestiques |\n\n### Notation CIDR\n\n```\n192.168.1.0/24\n           ↑\n           24 bits de masque réseau → 256 adresses (254 hôtes)\n```\n\n| Notation | Masque | Hôtes disponibles |\n|---|---|---|\n| `/32` | `255.255.255.255` | 1 hôte (adresse unique) |\n| `/24` | `255.255.255.0` | 254 hôtes |\n| `/16` | `255.255.0.0` | 65 534 hôtes |\n| `/8`  | `255.0.0.0` | 16 millions d'hôtes |\n\n### Commandes utiles\n\n```bash\nip addr show eth0              # adresse IP de l'interface\nip route show                  # passerelle par défaut\ncat /etc/resolv.conf           # serveurs DNS configurés\nnslookup google.com            # résoudre un nom DNS\ndig +short google.com A        # résolution DNS précise\ncurl ifconfig.me               # ton IP publique\n```\n"
    },
    {
     "name": "Pare-feux et sécurité réseau",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "content": "## Pare-feux et sécurité réseau\n\nUn pare-feu filtre le trafic réseau selon des règles. Il est la première ligne de défense d'un système.\n\n### iptables / nftables\n\n```bash\n# Voir les règles actives\niptables -L -v -n\n\n# Bloquer une IP\niptables -A INPUT -s 1.2.3.4 -j DROP\n\n# Autoriser le trafic entrant sur le port 443\niptables -A INPUT -p tcp --dport 443 -j ACCEPT\n\n# Bloquer tout sauf SSH et HTTPS\niptables -P INPUT DROP\niptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT\niptables -A INPUT -p tcp --dport 22 -j ACCEPT\niptables -A INPUT -p tcp --dport 443 -j ACCEPT\n```\n\n### ufw — pare-feu simplifié (Ubuntu)\n\n```bash\nufw enable                     # activer\nufw status verbose             # voir l'état et les règles\nufw allow 22/tcp               # autoriser SSH\nufw allow 443                  # autoriser HTTPS\nufw deny 3306                  # bloquer MySQL depuis l'extérieur\nufw delete allow 80            # supprimer une règle\n```\n\n### Bonnes pratiques de sécurité réseau\n\n```bash\n# Désactiver la connexion SSH par mot de passe (clé uniquement)\n# /etc/ssh/sshd_config :\nPasswordAuthentication no\nPermitRootLogin no\nAllowUsers alice bob\n\n# Scanner son propre réseau pour détecter les ports ouverts\nnmap -sV -O 192.168.1.0/24\n\n# Vérifier les connexions actives suspectes\nss -tnp | grep ESTABLISHED\n```\n\n### fail2ban — protection anti-brute force\n\n```bash\n# Installer et activer\napt install fail2ban\nsystemctl enable --now fail2ban\n\n# Voir les IPs bannies\nfail2ban-client status sshd\n\n# Débannir une IP\nfail2ban-client set sshd unbanip 1.2.3.4\n```\n"
    },
    {
     "name": "Quiz Networking Basics",
     "type": "quiz",
     "inline": true,
     "prerequisites": [
      "modèle-tcp-ip-et-protocoles",
      "adressage-ip-et-sous-réseaux",
      "pare-feux-et-sécurité-réseau"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Combien de couches comporte le modèle TCP/IP ?",
       "answers": [
        {
         "id": "a",
         "text": "4",
         "correct": true
        },
        {
         "id": "b",
         "text": "7",
         "correct": false
        },
        {
         "id": "c",
         "text": "5",
         "correct": false
        },
        {
         "id": "d",
         "text": "3",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quel masque de sous-réseau correspond à la notation CIDR /24 ?",
       "answers": [
        {
         "id": "a",
         "text": "255.255.255.0",
         "correct": true
        },
        {
         "id": "b",
         "text": "255.255.0.0",
         "correct": false
        },
        {
         "id": "c",
         "text": "255.0.0.0",
         "correct": false
        },
        {
         "id": "d",
         "text": "255.255.255.128",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quel protocole est sans connexion et ne garantit pas la livraison des paquets ?",
       "answers": [
        {
         "id": "a",
         "text": "UDP",
         "correct": true
        },
        {
         "id": "b",
         "text": "TCP",
         "correct": false
        },
        {
         "id": "c",
         "text": "HTTP",
         "correct": false
        },
        {
         "id": "d",
         "text": "FTP",
         "correct": false
        }
       ]
      },
      {
       "id": "q4",
       "type": "single",
       "question": "Quelle commande affiche tous les ports TCP en écoute sur un système Linux ?",
       "answers": [
        {
         "id": "a",
         "text": "ss -tlnp",
         "correct": true
        },
        {
         "id": "b",
         "text": "ifconfig -a",
         "correct": false
        },
        {
         "id": "c",
         "text": "ip route show",
         "correct": false
        },
        {
         "id": "d",
         "text": "ping localhost",
         "correct": false
        }
       ]
      },
      {
       "id": "q5",
       "type": "single",
       "question": "À quoi sert le NAT (Network Address Translation) ?",
       "answers": [
        {
         "id": "a",
         "text": "Permettre à plusieurs machines d'un réseau privé de partager une seule IP publique",
         "correct": true
        },
        {
         "id": "b",
         "text": "Chiffrer le trafic entre deux points d'extrémité",
         "correct": false
        },
        {
         "id": "c",
         "text": "Attribuer dynamiquement des adresses IP aux hôtes",
         "correct": false
        },
        {
         "id": "d",
         "text": "Router les paquets entre des VLANs",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "networking-essentials",
  "spec": {
   "title": "Réseaux informatiques essentiels",
   "description": "Comprends les fondamentaux des réseaux : modèle OSI, TCP/IP, DNS, HTTP et les protocoles clés. Indispensable pour tout ingénieur DevOps ou développeur.",
   "category": "networking",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Expert réseau",
    "icon": "🔌"
   },
   "modules": [
    {
     "name": "Le modèle OSI",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/01-intro.md"
    },
    {
     "name": "TCP/IP en pratique",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/02-images.md",
     "prerequisites": [
      "le-modèle-osi"
     ]
    },
    {
     "name": "DNS et résolution de noms",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/03-networking.md",
     "prerequisites": [
      "tcp-ip-en-pratique"
     ]
    },
    {
     "name": "HTTP et HTTPS",
     "type": "text",
     "skills": [
      "reseaux"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/04-compose.md",
     "prerequisites": [
      "dns-et-résolution-de-noms"
     ]
    },
    {
     "name": "Quiz Réseaux",
     "type": "quiz",
     "skills": [
      "reseaux"
     ],
     "inline": true,
     "prerequisites": [
      "le-modèle-osi",
      "tcp-ip-en-pratique",
      "dns-et-résolution-de-noms",
      "http-et-https"
     ],
     "passingScore": 70,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Combien de couches compte le modèle OSI ?",
       "answers": [
        {
         "id": "a",
         "text": "7",
         "correct": true
        },
        {
         "id": "b",
         "text": "4",
         "correct": false
        },
        {
         "id": "c",
         "text": "5",
         "correct": false
        },
        {
         "id": "d",
         "text": "3",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quel protocole est utilisé pour la résolution de noms de domaine ?",
       "answers": [
        {
         "id": "a",
         "text": "DNS",
         "correct": true
        },
        {
         "id": "b",
         "text": "DHCP",
         "correct": false
        },
        {
         "id": "c",
         "text": "FTP",
         "correct": false
        },
        {
         "id": "d",
         "text": "SMTP",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Sur quel port HTTPS écoute-t-il par défaut ?",
       "answers": [
        {
         "id": "a",
         "text": "443",
         "correct": true
        },
        {
         "id": "b",
         "text": "80",
         "correct": false
        },
        {
         "id": "c",
         "text": "8080",
         "correct": false
        },
        {
         "id": "d",
         "text": "22",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "pentesting-basics",
  "spec": {
   "title": "Bases du pentest",
   "description": "Apprendre à penser comme un attaquant pour mieux défendre — méthodologie, reconnaissance, scanning et exploitation basique dans un cadre légal et éthique.",
   "category": "security",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Ethical Hacker",
    "icon": "🎯"
   },
   "modules": [
    {
     "name": "Méthodologie et cadre légal",
     "type": "text",
     "skills": [
      "pentesting"
     ],
     "content": "## Méthodologie et cadre légal\n\nUn pentest (penetration test) est une attaque simulée et **autorisée** sur un système pour en identifier les failles avant un vrai attaquant.\n\n### Cadre légal — règle absolue\n\n**Sans autorisation écrite explicite, toute tentative d'intrusion est illégale.**\n\n```\n✅ Autorisé :\n- Système qui t'appartient\n- CTF (Capture The Flag) — compétitions officielles\n- Bug bounty — programme déclaré (HackerOne, Intigriti…)\n- Mission de pentest avec contrat et scope défini\n\n❌ Illégal :\n- Tester un site \"pour voir\" sans autorisation\n- Scanner les réseaux de tiers\n- Exploiter une faille même pour \"prévenir\" le propriétaire\n```\n\n### Méthodologie PTES (Penetration Testing Execution Standard)\n\n```\n1. Pre-engagement    → définir le scope, les règles, les contacts d'urgence\n2. Reconnaissance   → collecter des informations passives et actives\n3. Threat modeling  → identifier les vecteurs d'attaque prioritaires\n4. Vulnerability ID → scanner et identifier les failles\n5. Exploitation     → exploiter les vulnérabilités dans le scope\n6. Post-exploitation → mesurer l'impact réel (pivot, persistence…)\n7. Reporting        → documenter et recommander des corrections\n```\n\n### Environnements de pratique légaux\n\n```bash\n# Vulnerable by design — à installer localement\ndocker run -d -p 3000:3000 ghcr.io/juice-shop/juice-shop  # OWASP Juice Shop\ndocker run -d -p 80:80 vulnerables/web-dvwa                # DVWA\n\n# Plateformes CTF / labs en ligne\n# HackTheBox, TryHackMe, Root-Me, PicoCTF\n```\n"
    },
    {
     "name": "Reconnaissance et scanning",
     "type": "text",
     "skills": [
      "pentesting"
     ],
     "content": "## Reconnaissance et scanning\n\n### Reconnaissance passive (OSINT)\n\nCollecter des informations sans toucher directement la cible.\n\n```bash\n# Recherche de sous-domaines (DNS)\nsubfinder -d example.com\namass enum -d example.com\n\n# Google Dorks — requêtes Google avancées\nsite:example.com filetype:pdf\nsite:example.com inurl:admin\n\"example.com\" \"password\" filetype:txt\n\n# Shodan — moteur de recherche pour appareils connectés\n# shodan.io → chercher une IP ou un banner de service\n\n# Certificate Transparency Logs\ncurl \"https://crt.sh/?q=%.example.com&output=json\" | jq '.[].name_value'\n```\n\n### Scanning actif — Nmap\n\n```bash\n# Scan des ports les plus courants\nnmap -sC -sV -oA scan-result target.example.com\n\n# Scan de tous les ports TCP\nnmap -p- --min-rate 5000 target.example.com\n\n# Détection d'OS et de services\nnmap -O -sV target.example.com\n\n# Options courantes\n# -sC  : scripts par défaut (safe)\n# -sV  : détection de version des services\n# -oA  : output en 3 formats (normal, XML, grepable)\n# -p-  : tous les 65535 ports\n```\n\nExemple de sortie :\n\n```\nPORT    STATE SERVICE  VERSION\n22/tcp  open  ssh      OpenSSH 8.9p1\n80/tcp  open  http     nginx 1.24.0\n443/tcp open  ssl/http nginx 1.24.0\n5432/tcp closed postgresql\n```\n\n### Enumération web\n\n```bash\n# Gobuster — brute-force de répertoires/fichiers\ngobuster dir -u https://target.example.com \\\n  -w /usr/share/wordlists/dirb/common.txt \\\n  -x php,html,js,json\n\n# Feroxbuster — récursif\nferoxbuster -u https://target.example.com \\\n  -w /usr/share/seclists/Discovery/Web-Content/raft-medium-directories.txt\n```\n"
    },
    {
     "name": "Exploitation basique et Burp Suite",
     "type": "text",
     "skills": [
      "pentesting"
     ],
     "content": "## Exploitation basique et Burp Suite\n\n### Burp Suite — proxy d'interception\n\nBurp Suite Community (gratuit) s'intercale entre ton navigateur et la cible pour capturer et modifier les requêtes HTTP.\n\n```\nNavigateur → Burp Proxy (127.0.0.1:8080) → Serveur cible\n```\n\nFonctionnalités clés :\n- **Proxy** : intercepter et modifier les requêtes\n- **Repeater** : rejouer une requête modifiée manuellement\n- **Intruder** : automatiser des attaques par dictionnaire (brute-force, fuzzing)\n- **Decoder** : encoder/décoder Base64, URL, HTML…\n- **Scanner** (Pro uniquement) : scan automatique de vulnérabilités\n\n### Tester une injection SQL manuellement\n\n```\n# Dans Burp Repeater — tester chaque paramètre\nGET /users?id=1         → réponse normale\nGET /users?id=1'        → erreur SQL ? (révèle la vulnérabilité)\nGET /users?id=1 OR 1=1  → tous les utilisateurs ?\nGET /users?id=1 AND SLEEP(5)-- → time-based blind SQLi\n```\n\n```bash\n# SQLMap — automatisation de l'injection SQL\n# (uniquement sur tes labs ou cibles autorisées)\nsqlmap -u \"http://localhost:3000/users?id=1\" --dbs\n```\n\n### OWASP Juice Shop — exercices pratiques\n\n```bash\ndocker run -d -p 3000:3000 ghcr.io/juice-shop/juice-shop\n\n# Challenges accessibles à localhost:3000\n# Niveau 1 : trouver la page d'administration cachée\n# Niveau 2 : se connecter en tant qu'admin via SQLi\n# Niveau 3 : exécuter un XSS stocké\n# Niveau 4 : accéder aux commandes d'un autre utilisateur (IDOR)\n```\n\n### Le rapport de pentest\n\nUn pentest sans rapport n'a aucune valeur. Le rapport doit contenir :\n\n```markdown\n## Résumé exécutif\nSynthèse pour la direction (non technique)\n\n## Vulnérabilités trouvées\n| ID | Titre | Sévérité | CVSS | Statut |\n|----|-------|----------|------|--------|\n| V01 | SQL Injection sur /login | Critique | 9.8 | Ouvert |\n\n## Détail de chaque vulnérabilité\n- Description\n- Étapes de reproduction (proof of concept)\n- Impact réel\n- Recommandation de correction\n\n## Remerciements et périmètre testé\n```\n"
    },
    {
     "name": "Quiz — Bases du pentest",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "méthodologie-et-cadre-légal",
      "reconnaissance-et-scanning",
      "exploitation-basique-et-burp-suite"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle condition est obligatoire avant de réaliser un pentest ?",
       "answers": [
        {
         "id": "a",
         "text": "Avoir une autorisation écrite explicite du propriétaire du système",
         "correct": true
        },
        {
         "id": "b",
         "text": "Utiliser uniquement des outils open source",
         "correct": false
        },
        {
         "id": "c",
         "text": "Informer les autorités locales avant de commencer",
         "correct": false
        },
        {
         "id": "d",
         "text": "Ne pas exploiter les vulnérabilités, seulement les identifier",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Qu'est-ce que la reconnaissance passive (OSINT) ?",
       "answers": [
        {
         "id": "a",
         "text": "Collecter des informations sur la cible sans interagir directement avec elle",
         "correct": true
        },
        {
         "id": "b",
         "text": "Scanner les ports de la cible avec Nmap",
         "correct": false
        },
        {
         "id": "c",
         "text": "Tester les formulaires web pour des injections",
         "correct": false
        },
        {
         "id": "d",
         "text": "Capturer le trafic réseau de la cible",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "À quoi sert le Repeater dans Burp Suite ?",
       "answers": [
        {
         "id": "a",
         "text": "Rejouer et modifier manuellement une requête HTTP capturée",
         "correct": true
        },
        {
         "id": "b",
         "text": "Automatiser des attaques par dictionnaire sur un paramètre",
         "correct": false
        },
        {
         "id": "c",
         "text": "Scanner automatiquement toutes les vulnérabilités du site",
         "correct": false
        },
        {
         "id": "d",
         "text": "Intercepter les requêtes entre le navigateur et le serveur",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "python-basics",
  "spec": {
   "title": "Python pour les débutants",
   "description": "Apprends Python depuis les fondamentaux : variables, structures de données, fonctions et programmation orientée objet. Le cours idéal pour débuter en programmation.",
   "category": "programming",
   "difficulty": "beginner",
   "public": true,
   "badge": {
    "name": "Pythoniste",
    "icon": "🐍"
   },
   "modules": [
    {
     "name": "Introduction à Python",
     "type": "text",
     "skills": [
      "python"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/01-intro.md"
    },
    {
     "name": "Variables et types",
     "type": "text",
     "skills": [
      "python"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/02-images.md",
     "prerequisites": [
      "introduction-à-python"
     ]
    },
    {
     "name": "Structures de contrôle",
     "type": "text",
     "skills": [
      "python"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/03-networking.md",
     "prerequisites": [
      "variables-et-types"
     ]
    },
    {
     "name": "Fonctions et modules",
     "type": "text",
     "skills": [
      "python"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/04-compose.md",
     "prerequisites": [
      "structures-de-contrôle"
     ]
    },
    {
     "name": "Listes, tuples et dictionnaires",
     "type": "text",
     "skills": [
      "python"
     ],
     "src": "https://github.com/paulmascarilla/course-example",
     "ref": "main",
     "path": "lessons/04-compose.md",
     "prerequisites": [
      "fonctions-et-modules"
     ]
    },
    {
     "name": "Quiz Python Basics",
     "type": "quiz",
     "skills": [
      "python"
     ],
     "inline": true,
     "prerequisites": [
      "introduction-à-python",
      "variables-et-types",
      "structures-de-contrôle",
      "fonctions-et-modules",
      "listes--tuples-et-dictionnaires"
     ],
     "passingScore": 75,
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Quelle est la syntaxe correcte pour afficher 'Hello World' en Python ?",
       "answers": [
        {
         "id": "a",
         "text": "print('Hello World')",
         "correct": true
        },
        {
         "id": "b",
         "text": "echo 'Hello World'",
         "correct": false
        },
        {
         "id": "c",
         "text": "console.log('Hello World')",
         "correct": false
        },
        {
         "id": "d",
         "text": "printf('Hello World')",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Comment définit-on une fonction en Python ?",
       "answers": [
        {
         "id": "a",
         "text": "def ma_fonction():",
         "correct": true
        },
        {
         "id": "b",
         "text": "function ma_fonction():",
         "correct": false
        },
        {
         "id": "c",
         "text": "fun ma_fonction():",
         "correct": false
        },
        {
         "id": "d",
         "text": "void ma_fonction():",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Quel est le type de données pour une séquence ordonnée et modifiable en Python ?",
       "answers": [
        {
         "id": "a",
         "text": "list",
         "correct": true
        },
        {
         "id": "b",
         "text": "tuple",
         "correct": false
        },
        {
         "id": "c",
         "text": "set",
         "correct": false
        },
        {
         "id": "d",
         "text": "dict",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 },
 {
  "slug": "secrets-management",
  "spec": {
   "title": "Gestion des secrets",
   "description": "Ne commite plus jamais un mot de passe. Découvre comment protéger clés API, tokens et credentials dans tes projets et pipelines CI/CD.",
   "category": "security",
   "difficulty": "intermediate",
   "public": true,
   "badge": {
    "name": "Gardien des secrets",
    "icon": "🔑"
   },
   "modules": [
    {
     "name": "Le danger des secrets dans le code",
     "type": "text",
     "skills": [
      "securite",
      "devops"
     ],
     "content": "## Le danger des secrets dans le code\n\nUn **secret** est toute donnée qui donne accès à quelque chose : mot de passe, clé API, token OAuth, certificat privé.\n\n### Pourquoi c'est critique\n\n```bash\n# Ce genre de code se retrouve partout sur GitHub\nDB_PASSWORD = \"supersecret123\"\nAWS_SECRET_ACCESS_KEY = \"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\"\n```\n\nEn 2023, plus de 10 millions de secrets ont été détectés sur GitHub. Une clé AWS exposée quelques minutes peut suffire à générer des milliers d'euros de charges.\n\n### L'historique Git ne pardonne pas\n\n```bash\n# Même si tu supprimes le secret dans un nouveau commit...\ngit log --all --full-history -- config.py\ngit show abc1234:config.py   # le secret est toujours là\n```\n\nUn `git push --force` ne supprime pas les forks ou les clones existants.\n\n### Méthodes de détection\n\n```bash\n# Trufflesecurity/trufflehog — scan ton repo\ntrufflehog git file://. --only-verified\n\n# GitLeaks — intégrable en pre-commit hook\ngitleaks detect --source .\n\n# GitHub Advanced Security — scan automatique sur push\n```\n\n### Règle n°1 : les secrets n'ont jamais leur place dans le code ou le dépôt.\n"
    },
    {
     "name": "Variables d'environnement et fichiers .env",
     "type": "text",
     "skills": [
      "securite",
      "devops"
     ],
     "content": "## Variables d'environnement et fichiers .env\n\n### Le pattern .env\n\n```bash\n# .env — jamais commité, listé dans .gitignore\nDATABASE_URL=postgres://user:pass@localhost:5432/mydb\nJWT_SECRET=une_chaine_aleatoire_de_32_caracteres\nSTRIPE_SECRET_KEY=sk_live_...\n```\n\n```bash\n# .env.example — commité, valeurs fictives\nDATABASE_URL=postgres://user:password@localhost:5432/mydb\nJWT_SECRET=changeme\nSTRIPE_SECRET_KEY=sk_test_...\n```\n\n```bash\n# .gitignore\n.env\n.env.local\n.env.production\n```\n\n### Kubernetes Secrets\n\n```yaml\napiVersion: v1\nkind: Secret\nmetadata:\n  name: db-credentials\ntype: Opaque\nstringData:\n  username: myuser\n  password: supersecret123\n```\n\n```yaml\n# Utilisation dans un Pod\nenv:\n  - name: DB_PASSWORD\n    valueFrom:\n      secretKeyRef:\n        name: db-credentials\n        key: password\n```\n\n> Les Secrets Kubernetes sont encodés en base64, pas chiffrés. Pour du chiffrement at-rest, activer `EncryptionConfiguration` ou utiliser un KMS externe.\n\n### Génération de secrets sécurisés\n\n```bash\n# 32 octets aléatoires en hexadécimal\nopenssl rand -hex 32\n\n# UUID v4\npython3 -c \"import uuid; print(uuid.uuid4())\"\n```\n"
    },
    {
     "name": "HashiCorp Vault",
     "type": "text",
     "skills": [
      "securite",
      "devops"
     ],
     "content": "## HashiCorp Vault\n\nVault est un gestionnaire de secrets centralisé : stockage chiffré, accès audité, rotation automatique.\n\n### Concepts clés\n\n| Concept | Description |\n|---|---|\n| **Secret Engine** | Backend de stockage (KV, base de données, PKI…) |\n| **Auth Method** | Comment s'authentifier (token, AppRole, Kubernetes…) |\n| **Policy** | Ce que tu peux lire/écrire |\n| **Lease** | Durée de vie d'un secret dynamique |\n\n### Secrets statiques (KV v2)\n\n```bash\n# Écrire un secret\nvault kv put secret/myapp db_password=\"supersecret\"\n\n# Lire un secret\nvault kv get secret/myapp\n\n# Lire un champ précis (utile en CI/CD)\nvault kv get -field=db_password secret/myapp\n```\n\n### Secrets dynamiques (base de données)\n\nVault génère des credentials éphémères à la demande et les révoque automatiquement.\n\n```bash\n# Vault crée un user PostgreSQL temporaire avec un TTL\nvault read database/creds/my-role\n# Key               Value\n# username          v-app-xyz123\n# password          A1B2C3D4...\n# lease_duration    1h\n```\n\nAprès 1h, le compte est automatiquement supprimé de PostgreSQL.\n\n### Intégration GitLab CI/CD\n\n```yaml\n# .gitlab-ci.yml\nvariables:\n  VAULT_ADDR: \"https://vault.example.com\"\n\ndeploy:\n  id_tokens:\n    VAULT_ID_TOKEN:\n      aud: vault\n  secrets:\n    DB_PASSWORD:\n      vault: secret/myapp/db_password@secret\n  script:\n    - echo \"Le secret est disponible comme $DB_PASSWORD\"\n```\n"
    },
    {
     "name": "Quiz — Gestion des secrets",
     "type": "quiz",
     "inline": true,
     "passingScore": 75,
     "prerequisites": [
      "le-danger-des-secrets-dans-le-code",
      "variables-d-environnement-et-fichiers--env",
      "hashicorp-vault"
     ],
     "questions": [
      {
       "id": "q1",
       "type": "single",
       "question": "Un développeur supprime un mot de passe d'un fichier et crée un nouveau commit. Le secret est-il effacé du dépôt ?",
       "answers": [
        {
         "id": "a",
         "text": "Non, il reste dans l'historique Git",
         "correct": true
        },
        {
         "id": "b",
         "text": "Oui, le nouveau commit écrase l'ancien",
         "correct": false
        },
        {
         "id": "c",
         "text": "Oui, si on fait un git push",
         "correct": false
        },
        {
         "id": "d",
         "text": "Non, mais un git gc le supprime",
         "correct": false
        }
       ]
      },
      {
       "id": "q2",
       "type": "single",
       "question": "Quelle est la différence entre un secret statique et un secret dynamique dans Vault ?",
       "answers": [
        {
         "id": "a",
         "text": "Le secret dynamique est généré à la demande avec une durée de vie limitée",
         "correct": true
        },
        {
         "id": "b",
         "text": "Le secret dynamique est chiffré, pas le statique",
         "correct": false
        },
        {
         "id": "c",
         "text": "Le secret statique est automatiquement rotaté toutes les 24h",
         "correct": false
        },
        {
         "id": "d",
         "text": "Il n'y a pas de différence fonctionnelle",
         "correct": false
        }
       ]
      },
      {
       "id": "q3",
       "type": "single",
       "question": "Que doit-on commiter dans un dépôt Git ?",
       "answers": [
        {
         "id": "a",
         "text": "Un fichier .env.example avec des valeurs fictives",
         "correct": true
        },
        {
         "id": "b",
         "text": "Le fichier .env avec les vraies valeurs",
         "correct": false
        },
        {
         "id": "c",
         "text": "Les secrets chiffrés en base64",
         "correct": false
        },
        {
         "id": "d",
         "text": "Les secrets dans un dossier .secrets/ gitignored",
         "correct": false
        }
       ]
      }
     ]
    }
   ]
  }
 }
];

export const SEED_PATHS: PathDefinition[] = [
 {
  "slug": "parcours-devops",
  "spec": {
   "title": "Parcours DevOps",
   "description": "Maîtrise les outils et pratiques DevOps modernes, de la conteneurisation à l'orchestration en passant par l'intégration continue.",
   "kind": "course",
   "level": "beginner",
   "courses": [
    "docker-fundamentals",
    "kubernetes-basics",
    "gitlab-cicd",
    "infrastructure-as-code",
    "monitoring-observability"
   ]
  }
 },
 {
  "slug": "parcours-securite",
  "spec": {
   "title": "Parcours Sécurité",
   "description": "Développe tes compétences en cybersécurité, des fondamentaux jusqu'aux techniques offensives et défensives.",
   "kind": "course",
   "level": "beginner",
   "courses": [
    "cybersecurity-intro",
    "secrets-management",
    "container-security",
    "devsecops",
    "pentesting-basics"
   ]
  }
 }
];
