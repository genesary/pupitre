---
slug: git-basics
title: Les bases de Git
description: Suivre ses fichiers, créer des branches et partager son travail.
category: git
difficulty: beginner
public: true
split: h2
---

# Les bases de Git

Git garde l'historique de vos fichiers et permet de travailler à plusieurs.

## Premier dépôt

```sh
git init
git add .
git commit -m "docs: first commit"
```

## Les branches

Une branche est une ligne de travail indépendante.

```sh
git switch -c feature/menu
```

## Quiz

<!--pupitre
type: quiz
passingScore: 50
questions:
  - id: q1
    type: single
    points: 1
    question: Quelle commande crée une branche et s'y place ?
    answers:
      - id: a
        text: git switch -c
        correct: true
      - id: b
        text: git commit
-->
