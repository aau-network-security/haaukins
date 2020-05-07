<template>
    <tr v-bind:class="{'bg-isUser': team.is_user}">
        <td v-if="pos < 4" class="text-center rank-col" v-bind:class="get_background(pos, team.is_user)">
            <span class="icon" v-bind:class="{ 'has-text-warning': pos === 1, 'is-silver': pos === 2, 'is-bronze': pos === 3 }">
              <i class="fas fa-trophy"></i>
            </span>
        </td>
        <td v-else class="text-center rank-col" v-bind:class="get_background(pos, team.is_user)">{{ pos }}</td>
        <td class="team-col" v-bind:class="get_background(pos, team.is_user)"><strong>{{ team.name }}</strong></td>
        <td class="text-center score-col" v-bind:class="get_background(pos, team.is_user)">{{ team.tpoints }}</td>
        <challenge-cell v-for="comp in team.completions" v-bind:key="comp" :completed="comp != null"  v-bind:class="get_background(pos, team.is_user)"></challenge-cell>
    </tr>
</template>

<script>
    import ChallengeCell from './ChallengeCell.vue'

    export default {
        name: 'team-row',
        props: {
            team: Object,
            pos: Number,
        },
        components: {
            ChallengeCell,
        },
        methods: {
            get_background(index, user){
                if (user){
                    return 'bg-isUser'
                }
                if (index % 2 === 0){
                    return 'even'
                }
                return 'odd'
            }
        }
    }
</script>

<style>
    .even {
        background-color: #ffffff;
    }
    .odd {
        background-color: rgb(233, 235, 245);
    }
    .bg-isUser {
        background-color: rgb(195, 203, 228)!important;
    }
</style>