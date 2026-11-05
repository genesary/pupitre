---
slug: demo-aide-memoire-git
title: "Aide-mémoire Git"
description: "Cours de démonstration pour tester l'import markdown sans découpage : le document entier forme un seul module."
category: git
difficulty: beginner
public: true
split: none
---

# Aide-mémoire Git

Les commandes Git du quotidien, réunies sur une page. Comme la frontmatter
déclare `split: none`, **tout ce document devient un seul module** : les
titres `##` ci-dessous restent du contenu et ne coupent rien. Le nom du
module est repris du titre `# Aide-mémoire Git`.

## Configuration initiale

```bash
git config --global user.name "Ada Lovelace"
git config --global user.email "ada@example.com"
git config --global init.defaultBranch main
git config --global pull.rebase true
```

## Travail quotidien

```bash
git status                 # voir l'état de la copie de travail
git add -p                 # indexer morceau par morceau
git commit -m "feat: ..."  # enregistrer un instantané
git pull --rebase          # récupérer sans commit de fusion
git push                   # publier ses commits
```

## Annuler proprement

| Situation | Commande |
|---|---|
| Modification non indexée | `git restore <fichier>` |
| Modification indexée | `git restore --staged <fichier>` |
| Dernier commit, garder les changements | `git reset --soft HEAD~1` |
| Dernier commit, tout jeter | `git reset --hard HEAD~1` |
| Commit déjà poussé | `git revert <sha>` |

## Branches

```bash
git switch -c feat/ma-fonction   # créer et basculer
git switch main                  # revenir sur main
git merge feat/ma-fonction       # fusionner dans la branche courante
git branch -d feat/ma-fonction   # supprimer une fois fusionnée
```

## Inspecter l'historique

```bash
git log --oneline --graph --decorate -20
git show <sha>
git blame <fichier>
git diff main...HEAD
```
