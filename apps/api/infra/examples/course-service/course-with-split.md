---
slug: demo-conteneurs
title: "Découverte des conteneurs"
description: "Cours de démonstration pour tester l'import markdown avec découpage : quatre modules texte et un module quiz, coupés au niveau h2."
category: conteneurs
difficulty: beginner
public: true
split: h2
---

## Pourquoi des conteneurs ?

Un **conteneur** empaquette une application avec tout ce dont elle a besoin
pour tourner — bibliothèques, binaires, fichiers de configuration — et
l'isole du reste du système. Contrairement à une machine virtuelle, il
partage le noyau de l'hôte : il démarre en quelques millisecondes et pèse
quelques mégaoctets.

### Le problème résolu

| Avant | Avec des conteneurs |
|---|---|
| « Ça marche sur ma machine » | La même image tourne partout |
| Dépendances installées à la main sur chaque serveur | Dépendances figées dans l'image |
| Montée de version risquée | On remplace l'image, on redémarre |

## Images et couches

<!--pupitre
skills: [docker, images]
-->

Une **image** est un modèle en lecture seule : une pile de couches
empilées, chacune correspondant à une instruction du `Dockerfile`. Les
couches sont mises en cache et partagées entre images.

```dockerfile
FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl
COPY ./app /app
CMD ["/app/start.sh"]
```

Quand on lance un conteneur, le moteur ajoute par-dessus une **couche
inscriptible** propre à ce conteneur : deux conteneurs issus de la même
image ne partagent jamais leurs écritures.

## Cycle de vie d'un conteneur

```bash
docker run -d --name web -p 8080:80 nginx   # créer + démarrer
docker ps                                    # lister les conteneurs actifs
docker logs -f web                           # suivre la sortie standard
docker exec -it web sh                        # ouvrir un shell dedans
docker stop web && docker rm web             # arrêter puis supprimer
```

Un conteneur arrêté garde sa couche inscriptible jusqu'à sa suppression :
`docker start web` le relance dans l'état où il était.

## Réseau et volumes

<!--pupitre
type: text
src: https://github.com/genesary/pupitre-courses
ref: main
path: conteneurs/reseau-et-volumes.md
-->

## Quiz — les bases

<!--pupitre
type: quiz
passingScore: 75
maxAttemptsPerQuestion: 2
skills: [docker]
questions:
  - id: q1
    type: single
    difficulty: easy
    points: 1
    question: "Qu'est-ce qu'une image de conteneur ?"
    answers:
      - {id: a, text: "Un modèle en lecture seule servant à créer des conteneurs", correct: true}
      - {id: b, text: "Un conteneur en cours d'exécution", correct: false}
      - {id: c, text: "Une machine virtuelle allégée", correct: false}
    feedback:
      correct: "Exact : l'image est le modèle, le conteneur en est une instance."
      wrong: "Revois la différence entre l'image (le modèle) et le conteneur (l'instance)."
  - id: q2
    type: boolean
    difficulty: easy
    points: 1
    question: "Deux conteneurs lancés depuis la même image partagent la même couche inscriptible."
    correctAnswer: false
    feedback:
      correct: "La couche inscriptible est propre à chaque conteneur."
      wrong: "Chaque conteneur reçoit sa propre couche inscriptible au démarrage."
  - id: q3
    type: multiple
    difficulty: medium
    points: 2
    question: "Quelles commandes créent ou (re)démarrent un conteneur ?"
    answers:
      - {id: a, text: "docker run", correct: true}
      - {id: b, text: "docker start", correct: true}
      - {id: c, text: "docker build", correct: false}
      - {id: d, text: "docker pull", correct: false}
    partialScoring: {enabled: true, allowNegative: false}
    feedback:
      correct: "run crée puis démarre, start relance un conteneur arrêté."
      wrong: "build fabrique une image et pull la télécharge — aucun des deux ne lance de conteneur."
-->
