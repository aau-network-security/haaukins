<template>
    <div id="challenges-board">
        <div class="row mt-5" v-for="category in challenges" v-bind:key="category[0].category">
            <div class="category-header col-md-12 mb-3">
                <h3>{{category[0].category}}</h3>
            </div>
            <div class="col-lg-3 col-md-4" v-for="challenge in category" v-bind:key="challenge.tag">
                <button class="btn btn-dark challenge-button w-100 text-truncate pt-3 pb-3 mb-2" v-on:click="openModal(challenge)">
                    <p>{{challenge.name}}</p>
                    <span>{{challenge.points}}</span>
                </button>
            </div>
        </div>
        <challenge-modal :challenge="this.modalInfo"></challenge-modal>
    </div>
</template>

<script>
    import ChallengeModal from "./ChallengeModal";

    export default {
        name: "ChallegesPage",
        components: {ChallengeModal},
        data: function () {
            return {
                modalInfo: {}, //passed to the modal
                challenges: {
                    "Web Exploitation": [],
                    "Forensics": [],
                    "Reverse Engineering": [],
                    "Binary": [],
                    "Cryptography": []
                },
                dummyChallenges: {
                    "challenges": [
                        {
                            "tag": "xss",
                            "name": "cross site scripting",
                            "points": 10,
                            "category": "Reverse Engineering",
                            "description": "bla bla bla"
                        },
                        {
                            "tag": "hb",
                            "name": "heart blead",
                            "points": 140,
                            "category": "Web Exploitation",
                            "description": "cia cia cia"
                        },
                        {
                            "tag": "bho",
                            "name": "bla bla blaaaaa",
                            "points": 51,
                            "category": "Forensics",
                            "description": "noo ono ono ono"
                        },
                        {
                            "tag": "bho",
                            "name": "nha nha nah",
                            "points": 1,
                            "category": "Reverse Engineering",
                            "description": "noo ono ono ono"
                        },
                        {
                            "tag": "bho",
                            "name": "nha nha nah",
                            "points": 1,
                            "category": "Binary",
                            "description": "noo ono ono ono"
                        },
                        {
                            "tag": "bho",
                            "name": "nha nha nah",
                            "points": 1,
                            "category": "Cryptography",
                            "description": "noo ono ono ono"
                        }
                    ]
                }
            }
        },
        created: function() {
            this.sortingChallenges()
        },
        methods: {
            sortingChallenges: function(){
                //method elaborate the challenges retrieved by amigo backend in order to display them correctly
                //create subobj based on challenges categories
                //sort each categories from the ones with lower points to the max (apply the proper algorithm)

                this.dummyChallenges.challenges.forEach(function (el) {
                    this.challenges[el.category].push(el)
                }, this);

            },
            openModal: function (obj) {
                window.console.log(obj)
                this.modalInfo = obj
                this.$bvModal.show('challengeModal')
            }
        }
    }
</script>

<style scoped>

</style>