# Secrets Store CSI Driver Project Membership

> Note: This document is a work in progress

> Original doc: [Secrets Store CSI Driver Project Membership](https://docs.google.com/document/d/1YqFVTJdxvNXhXkWogYf7AOKMnUjPFHCPuYtTgJSHohU/edit?usp=sharing)

## Overview

Hello! We're excited to have you contribute to Secrets Store CSI Driver sig-auth subproject! This document outlines the various roles for the Secrets Store CSI Driver sig-auth subproject, along with the responsibilites and privileges that come with them. Community members generally start at the first level of membership and advance up it as their involvement in the project grows. Our project members are happy to help you advance along the contributor ladder.

Each of the contributor roles below is organized into lists of three types of things:
* "Responsibilities" are things that a contributor is expected to do
* "Requirements" are qualifications a contributor needs to meet to be in that role
* "Privileges" are things a contributor on that level is entitled to

### Community Participant

Description: A Community Participant participates in the community and contributes their time, thoughts, etc. 

* Responsibilities include: 
  * Following the [CNCF CoC](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)
* How users can get involved with the community:
  * Participating in discussions in GitHub, Slack, and meetings
  * Helping other users
  * Submitting bug reports
  * Trying out new releases
  * Attending community events
  * Talking about the project on social media, blogs, and talks

### Contributor

Description: A Contributor contributes directly to the project and adds value to it. Contributions need not be code. People at the Contributor level may be new contributors, or they may only contribute occasionally.

* Responsibilities include:
  * Following the [CNCF CoC](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)
  * Following the project Contributing Guide (to be updated)
* Requirements (one or several of the below):
  * Reports, and sometimes resolves issues
  * Occasionally submits PRs
  * Contributes to the documentation
  * Regularly shows up at meetings, takes notes
  * Answers questions from other community members
  * Submits feedback on issues and PRs
  * Tests releases and patches and submits reviews
  * Runs or helps run events
  * Promotes the project in public
  * Helps run the project infrastructure
* Privileges:
  * Invitations to Contributor events
  * Eligible to become a Reviewer or Approver

### Reviewer

Description: Reviewers are Contributors who are able to review code for quality and correctness on the Secrets Store CSI Driver. They are knowledgeable about the codebase and software engineering principles. A Reviewer has the rights, responsiblities, and requirements of a Contributor, plus:

* Responsibilities include:
  * Running tests for PRs from members who aren’t part of Kubernetes org
    * This involves running ok-to-test after assessing the PR briefly 
  * Project quality control via code reviews
    * Focusing on code quality and correctness, including testing and factoring
    * May also review for more holistic issues, but not a requirement
  * Responding to review requests as per [community expectations](https://github.com/kubernetes/community/blob/master/contributors/guide/expectations.md)
  * Assigned PRs to review
  * Assigned test bugs

* Requirements (one or several of the below):
  * Reviewers entry in the [OWNERS](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/v0.0.22/OWNERS#L5) file
  * Membership in kubernetes-sigs org
  * Reviewer for at least 5 PRs to the codebase
  * Reviewed or merged at least 10 substantial PRs to the codebase
  * Knowledgeable about the codebase
  * Sponsored by a subproject approver
    * With no objections from other approvers
    * Done through PR to update the OWNERS file
  * May either self-nominate, be nominated by an approver in this subproject

* Privileges:
  * Invitations to Contributor events
  * Eligible to become a reviewer or approver

**Note:** Acceptance of code contributions requires at least one approver in addition to the assigned reviewers.


### Approver

Description: Code approvers are able to both review and approve code contributions. While code review is focused on code quality and correctness, approval is focused on holistic acceptance of a contribution including: backwards / forwards compatibility, adhering to API and flag conventions, subtle performance and correctness issues, interactions with other parts of the system, etc. An Approver has the rights, responsiblities, and requirements of a reviewer, plus:

* Responsibilities include:
  * Approvers entry in the [OWNERS](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/v0.0.22/OWNERS#L1) file
  * Approver status may be a precondition to accepting large code contributions
  * Project quality control via code reviews
    * Focusing on holistic acceptance of contribution such as dependencies with other features, backwards / forwards compatibility, API and flag definitions, etc
  * Meeting expectations to be responsive to review requests as per community expectations
  * Mentoring contributors and reviewers

* Requirements (one or several of the below):
  * Demonstrate sound technical judgement
  * Reviewer of the codebase for at least 3 months
  * Primary reviewer for at least 10 substantial PRs to the codebase
  * Reviewed or merged at least 30 PRs to the codebase
  * Nominated by a subproject owner
    * With no objections from other subproject owners
    * Done through PR to update the top-level OWNERS file

* Privileges:
  * May approve code contributions for acceptance
  * Approver status may be a precondition to accepting large code contributions

### Maintainer

Description: Maintainers are very established contributors who are responsible for the entire project. As such, they have the ability to review and approve PRs against any area of the project, and are expected to participate in making decisions about the strategy and priorities of the project.

A Maintainer has the rights, responsiblities, and requirements of an Approver, plus:

* Responsibilities include:
  * Reviewing and approving PRs that involve multiple parts of the project
  * Is supportive of new and infrequent contributors, and helps get useful PRs in shape to commit
  * Mentoring new Maintainers
  * Writing refactoring PRs
  * Participating in CNCF Maintainer activities
  * Determining strategy and policy for the project
  * Participating in, and leading, community meetings
* Requirements
  * Experience as an Approver for at least 6 months   
  * Demonstrates a broad knowledge of the project across multiple areas
  * Is able to exercise judgement for the good of the project, independant of their employer, social circles, or teams
  * Mentors other Contributors
  
* Additional privileges:
  * Represent the project in public as a Maintainer
  * Communicate with the CNCF on behalf of the project   
  * Have a vote in Maintainer decisions

Process of becoming a maintainer:

1. Any current Maintainer may nominate a current Contributor to become a new Maintainer, by opening a PR against the root of the [Secret Store CSI Driver repository](https://github.com/kubernetes-sigs/secrets-store-csi-driver) and adding the nominee to the OWNERS file.
2. The nominee will add a comment to the PR testifying that they agree to all requirements of becoming a Maintainer.
3. A majority of the current Maintainers must then approve the PR.

## Inactivity

It is important for contributors to be and stay active to set an example and show commitment to the project. Inactivity is harmful to the project as it may lead to unexpected delays, contributor attrition, and a lost of trust in the project.

* Inactivity is measured by:
  * Periods of no contributions for longer than 6 months
  * Periods of no communication for longer than 6 months

* Consequences of being inactive include:
  * Involuntary removal or demotion
  * Being asked to move to Emeritus status

### Involuntary Removal or Demotion

Involuntary removal/demotion of a contributor happens when responsibilites and requirements aren't being met. This may include repeated pattern of inactivity, extended period of inactivity, a period of failing to meet the requirements of your role, and/or a violation of the [CNCF CoC](https://github.com/cncf/foundation/blob/master/code-of-conduct.md). This process is important because it protects the community and its deliverables while also opens up opportunities for new contributors to step in.


Involuntary removal or demotion is handled through a vote by a majority of the current Maintainers.

### Stepping Down/Emeritus Process

If and when contributors' commitment levels change, contributors can consider stepping down (moving down the contributor ladder) vs moving to emeritus status (completely stepping away from the project).

Contact the Maintainers about changing to Emeritus status, or reducing your contributor level.

## Contact

For inquiries, please reach out to [Secret Store CSI Driver Maintainers](TBD)


## Docs used for reference

- [Community membership](https://github.com/kubernetes/community/blob/master/community-membership.md)
